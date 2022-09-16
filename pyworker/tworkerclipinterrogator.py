# base on https://github.com/pharmapsychotic/clip-interrogator
# setup:
# git clone https://github.com/pharmapsychotic/clip-interrogator, copy tworkerclipinterrogator.py to clip-interrogator,
# cd clip-interrogator
# mkdir models
# git clone https://github.com/openai/CLIP 
# git clone https://github.com/salesforce/BLIP
# wget https://storage.googleapis.com/sfr-vision-language-research/BLIP/models/model*_base_caption.pth -O models/model_base_caption.pth

import os, sys, io, random, ssl, time, json, random
import logging, configparser
import threading
from threading import Lock
import pika
from PIL import Image
import numpy as np
import torch
import df_pb2

from torchvision import transforms
from torchvision.transforms.functional import InterpolationMode

root_path = os.getcwd()
sys.path.append(f'{root_path}/CLIP')
from CLIP import clip

sys.path.append(f'{root_path}/BLIP')
from models.blip import blip_decoder


tasklock = Lock()

logging.basicConfig(
    format="%(asctime)s %(message)s",
    stream=sys.stdout,
    level=logging.WARNING,
    filemode="w",
)

blip_image_eval_size = 384
device = torch.device('cuda:0')

def loadmodel(clipmodelnames):
    clipmodels = {}
    for model_name in models:
        print("load clip model:", model_name)
        model, preprocess = clip.load(model_name, device=device, jit=True)
        model = model.to(device)
        model.eval()
        clipmodels[model_name] = {"model": model,"preprocess": preprocess}
        print("model loaded: ", model_name)

    print("load blip model")
    blip_model_url = f'{root_path}/models/model_base_caption.pth'        
    blip_model = blip_decoder(pretrained=blip_model_url, med_config=f'{root_path}/BLIP/configs/med_config.json',image_size=blip_image_eval_size, vit='base')
    blip_model = blip_model.to(device)
    blip_model.eval()
    return blip_model, clipmodels


def generate_caption(blip_model , image):
    gpu_image = transforms.Compose([
        transforms.Resize((blip_image_eval_size, blip_image_eval_size), interpolation=InterpolationMode.BICUBIC),
        transforms.ToTensor(),
        transforms.Normalize((0.48145466, 0.4578275, 0.40821073), (0.26862954, 0.26130258, 0.27577711))
    ])(image).unsqueeze(0).to(device)

    with torch.no_grad():
        caption = blip_model.generate(gpu_image, sample=False, num_beams=3, max_length=20, min_length=5)
    return caption[0]

def load_list(filename):
    with open(filename, 'r', encoding='utf-8', errors='replace') as f:
        items = [line.strip() for line in f.readlines()]
    return items

def rank(model, image_features, text_array, top_count=1):
    top_count = min(top_count, len(text_array))
    text_tokens = clip.tokenize([text for text in text_array]).cuda()
    with torch.no_grad():
        text_features = model.encode_text(text_tokens).float()
    text_features /= text_features.norm(dim=-1, keepdim=True)

    similarity = torch.zeros((1, len(text_array))).to(device)
    for i in range(image_features.shape[0]):
        similarity += (100.0 * image_features[i].unsqueeze(0) @ text_features.T).softmax(dim=-1)
    similarity /= image_features.shape[0]

    top_probs, top_labels = similarity.cpu().topk(top_count, dim=-1)  
    return [(text_array[top_labels[0][i].numpy()], (top_probs[0][i].numpy()*100)) for i in range(top_count)]


class ThreadedConsumer(threading.Thread):
    def __init__(self, tid, taskqueuename, config):
        threading.Thread.__init__(self)
        self.tid=tid
        taskqueuekey = tid
        amqpurl = config["AMQP_URL"]
        parameters = pika.URLParameters(amqpurl)

        if "TLS_CA_CERT" in config:
            context = ssl.create_default_context(cafile=config["TLS_CA_CERT"])
            context.load_cert_chain(config["TLS_CLIENT_CERT"], config["TLS_CLIENT_KEY"])
            ssl_options = pika.SSLOptions(context, "localhost")
            parameters.ssl_options=ssl_options

        self.connection = pika.BlockingConnection(parameters)
        self.channel = self.connection.channel()
        self.channel.basic_qos(prefetch_count=1)
        qname = taskqueuename+'.'+taskqueuekey
        print("waiting tasks from {}".format(qname))
        self.channel.basic_consume(queue=qname, on_message_callback=self.callback, auto_ack=False)
        threading.Thread(target=self.channel.basic_consume(queue=qname, on_message_callback=self.callback, auto_ack=False))

    def callback(self, channel, method, properties, body):
        print("[x] Received from {}, body length {}".format(method.routing_key, len(body)))
        with tasklock:
            print(" [*] task start...")

            task = df_pb2.Task()
            task.ParseFromString(body)
            err, inputId, desctext = taskRunner(task)
            if err == "":
                print("interrogator task Succ, send ack.", inputId, task.TaskId)
                print("output text:", desctext)
                pbdata, nexttask = repacktask(inputId, desctext, task)
                #find next inputtask
                r = publish(pbdata, channel, nexttask.Name)
                if r == "":
                    logging.debug("interrogator task Succ, send ack.")
                    channel.basic_ack(method.delivery_tag)
                else:
                    logging.debug("interrogator task Err:", r)
                    channel.basic_ack(method.delivery_tag)
            else:
                print("interrogator task failure, send nack.", inputId, task.TaskId)
                print(err)
                channel.basic_ack(method.delivery_tag)

    def run(self):
        print("start thread:", self.tid)
        self.channel.start_consuming()

def loadconf():
    config = configparser.ConfigParser()
    config.read("configs/clipinterrogatorconfig.ini")
    return config


def taskRunner(task):
    outputlen = len(task.OutputList)

    if outputlen >= len(task.InputList) :
        return

    inputtask = task.InputList[outputlen]
    prevoutput = None
    if outputlen > 0 :
        prevoutput = task.OutputList[outputlen-1]

    ainame = inputtask.Name
    if ainame == 'ai.clipinterrogator':
        inputsettings = {}
        data = None
        if prevoutput != None:
            data = prevoutput.Data
        elif inputtask.Data != None:
            data = inputtask.Data 
        if data == None:
            return "ERR_NO_INPUT_IMAGE_DATA" , "", "" ,[] ,""

        dataStream = io.BytesIO(data)
        pilimage = Image.open(dataStream).convert('RGB')
        outputdesc = interrogate('ViT-L/14',pilimage)
        args = ""
        return "", inputtask.InputId, outputdesc
    else: 
        #not success, return errr and nack
        return "ERR_NOT_SUPPORT_MODEL" , "", ""
    

def publish(result, channel, taskname):
    l = taskname.split(".")
    if len(l)==3: #with route key
        key = l[2]
        exchange = l[0]+"."+l[1]
        channel.basic_publish(exchange=exchange, routing_key=key, body=result)
        return ""
    elif len(l)==2: #no special routekey, use all as route key
        exchange = l[0]+"."+l[1]
        channel.basic_publish(exchange=exchange, routing_key="all", body=result)
        return ""
    else: 
        return "ERR_INVAILD_TASKNAME"


def interrogate(modelname, image):
    caption = generate_caption(blipmodel, image)
    if len(models) == 0:
        print(f"\n\n{caption}")
        return

    table = []
    bests = [[('',0)]]*5
    clipmodel = clipmodels[modelname]

    preprocess = clipmodel["preprocess"]
    model = clipmodel["model"]

    images = preprocess(image).unsqueeze(0).cuda()
    with torch.no_grad():
        image_features = model.encode_image(images).float()
    image_features /= image_features.norm(dim=-1, keepdim=True)

    ranks = [
        rank(model, image_features, mediums),
        rank(model, image_features, ["by "+artist for artist in artists]),
        rank(model, image_features, trending_list),
        rank(model, image_features, movements),
        rank(model, image_features, flavors, top_count=3)
    ]

    for i in range(len(ranks)):
        confidence_sum = 0
        for ci in range(len(ranks[i])):
            confidence_sum += ranks[i][ci][1]
        if confidence_sum > sum(bests[i][t][1] for t in range(len(bests[i]))):
            bests[i] = ranks[i]

    ranktab = ""
    columns=["Medium", "Artist", "Trending", "Movement", "Flavors"]
    idx = 0
    for r in ranks:
        ranktab = ranktab + columns[idx] + ":" + ' '.join([f"{x[0]} ({x[1]:0.1f}%)" for x in r]) + " "
        idx = idx + 1

    flaves = ', '.join([f"{x[0]}" for x in bests[4]])
    medium = bests[0][0][0]
    outputdesc = ""
    if caption.startswith(medium):
        outputdesc = f"\n\n{caption} {bests[1][0][0]}, {bests[2][0][0]}, {bests[3][0][0]}, {flaves}"
    else:
        outputdesc =f"\n\n{caption} {medium} {bests[1][0][0]}, {bests[2][0][0]}, {bests[3][0][0]}, {flaves}"
    return ranktab + outputdesc
        
def repacktask(inputId, desctext, task):
    r = df_pb2.Output()
    r.InputId = inputId
    r.Version = 1
    r.ProducerName = cfg["WORKER"]["NAME"]
    r.ProducerSign = "mysig_taskid_userid"
    r.MimeType = "text/plain"
    r.Data = str.encode(desctext, "utf-8")

    if len(task.OutputList) > 0 : #remove lastoutput.Data
        last = task.OutputList[len(task.OutputList)-1]
        lastrd = df_pb2.Output()
        lastrd.InputId = last.InputId
        lastrd.Version = last.Version
        lastrd.ProducerName = last.ProducerName
        lastrd.ProducerSign = last.ProducerSign
        lastrd.MimeType = lastrd.MimeType
        #remove the Data
        lastrd.Args = last.Args
        task.OutputList.pop()
        task.OutputList.append(lastrd)

    task.OutputList.append(r)
    nexttask = task.InputList[len(task.OutputList)]
    output = task.SerializeToString()
    return output, nexttask

cfg = loadconf()
data_path = f'{root_path}/data/'
artists = load_list(os.path.join(data_path, 'artists.txt'))
flavors = load_list(os.path.join(data_path, 'flavors.txt'))
mediums = load_list(os.path.join(data_path, 'mediums.txt'))
movements = load_list(os.path.join(data_path, 'movements.txt'))

sites = ['Artstation', 'behance', 'cg society', 'cgsociety', 'deviantart', 'dribble', 'flickr', 'instagram', 'pexels', 'pinterest', 'pixabay', 'pixiv', 'polycount', 'reddit', 'shutterstock', 'tumblr', 'unsplash', 'zbrush central']
trending_list = [site for site in sites]
trending_list.extend(["trending on "+site for site in sites])
trending_list.extend(["featured on "+site for site in sites])
trending_list.extend([site+" contest winner" for site in sites])
models = ['ViT-L/14']
blipmodel, clipmodels = loadmodel(models)

if __name__ == "__main__":
    queueconfig = cfg["QUEUE"]
    workername = queueconfig["WORKER_NAME"]
    workerkeys = queueconfig["WORKER_KEYS"]

    keylist = workerkeys.split(",")

    try:
        for key in keylist:
            key = key.strip()
            td = ThreadedConsumer(key, workername, queueconfig)
            td.start()
        print("* Worker waiting for tasks. To exit press CTRL+C")

    except KeyboardInterrupt:
        print("* Exiting")
        try:
            sys.exit(0)
        except SystemExit:
            os._exit(0)

