import os, sys, io, random, ssl, time, json, random
import logging, configparser
import threading
from threading import Lock
import pika
from PIL import Image
import df_pb2

from torch import autocast
import torch
root_path = os.getcwd()
sys.path.append(f'{root_path}/diffusers/src')
from diffusers import StableDiffusionPipeline, LMSDiscreteScheduler


tasklock = Lock()

logging.basicConfig(
    format="%(asctime)s %(message)s",
    stream=sys.stdout,
    level=logging.WARNING,
    filemode="w",
)

def loadmodelpipe():

    lms = LMSDiscreteScheduler(
        beta_start=0.00085, 
        beta_end=0.012, 
        beta_schedule="scaled_linear"
    )
    
    model_id = "CompVis/stable-diffusion-v1-4"
    print("loading {}...".format(model_id))
    
    pipe = StableDiffusionPipeline.from_pretrained(
        model_id, 
        scheduler=lms,
        use_auth_token=True
    ).to("cuda")
    return pipe


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

            err, inputId, mime, data, args = taskRunner(task)
            if err == "":
                print("AI task Succ, send ack.", inputId, task.TaskId)
                pbdata, nexttask = repacktask(inputId, mime,  data, task, args)
                #find next inputtask
                r = publish(pbdata, channel, nexttask.Name)
                if r == "":
                    logging.debug("AI task Succ, send ack.")
                    channel.basic_ack(method.delivery_tag)
                else:
                    logging.debug("AI task Err:", r)
                    channel.basic_ack(method.delivery_tag)
            else:
                print("AI task failure, send nack.", inputId, task.TaskId)
                logging.debug("AI task failure, send nack")
                #channel.basic_nack(method.delivery_tag)
                channel.basic_ack(method.delivery_tag)

    def run(self):
        print("start thread:", self.tid)
        self.channel.start_consuming()

def loadconf():
    config = configparser.ConfigParser()
    config.read("configs/config.ini")
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
    if ainame == 'ai.sd14':
        inputsettings = json.loads(inputtask.Settings)
        r, mime, data, finalsettings = aiwork(inputsettings)
        print("image len:", len(data))
        args = settingsToOutput(inputsettings, finalsettings)
        return r, inputtask.InputId, mime, data, bytes(args, 'utf-8')
    else: 
        #not success, return errr and nack
        return "ERR_NOT_SUPPORT_MODEL" , "", "" ,[] ,""
    

def settingsToOutput(settings, finalsettings):
    output = settings["prompt"]
    if settings["height"] != 0:
        output = output + " -H "+ str(settings["height"])
    if settings["width"] != 0:
        output = output + " -W "+ str(settings["width"])
    if settings["guidance_scale"] != 0:
        output = output + " -C "+ str(settings["guidance_scale"])
    if settings["num_inference_steps"] != 0:
        output = output + " -s "+ str(settings["num_inference_steps"])
    if settings["seed"] != 0:
        output = output + " -S "+ str(settings["seed"])
    else:
        output = output + " -S "+ str(finalsettings["seed"])
    return output

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
        
def aiwork(inputsettings):
    settings = {"height":512, "width":512 , "num_inference_steps":50, "guidance_scale":7.5, "eta":0}
    if inputsettings['prompt']!="":
        settings['prompt'] = inputsettings['prompt']
    if inputsettings['height'] > 0 :
        settings['height'] = inputsettings['height']
    if inputsettings['width'] > 0 :
        settings['width'] = inputsettings['width']
    if inputsettings['num_inference_steps'] > 0 :
        settings['num_inference_steps'] = inputsettings['num_inference_steps']
    if inputsettings['guidance_scale'] > 0 :
        settings['guidance_scale'] = inputsettings['guidance_scale']
    if inputsettings['eta'] > 0 :
        settings['eta'] = inputsettings['eta']

    if 'seed' not in inputsettings or inputsettings['seed'] == 0 :
        inputsettings['seed'] = random.randint(1000000000, 9999999999)

    customgenerator = torch.Generator(device='cuda')
    customgenerator = customgenerator.manual_seed(inputsettings['seed'])
    settings['generator'] = customgenerator

    logging.debug("aiwork with settings:")
    logging.debug(settings)
    try:
        with autocast("cuda"):
            image = pipe(**settings)["sample"][0]  
        img_byte_arr = io.BytesIO()
        image.save(img_byte_arr, format='PNG')
        img_byte_arr = img_byte_arr.getvalue()

        settings.pop('generator', None)
        settings['seed']= inputsettings['seed']
        return "", "image/png", img_byte_arr, settings
    except Exception as e:
      print(e)
      return "ERR_AIWORK_FAILURE", "", [], {}

def fakeaiwork(settings):
    print("*** fake ai working***")
    print(settings)
    time.sleep(5)
    r = random.random()
    if r < 0.5:
        #fake result image
        filename = "output.png"
        image = Image.open(filename)
        img_byte_arr = io.BytesIO()
        image.save(img_byte_arr, format='PNG')
        img_byte_arr = img_byte_arr.getvalue()
        return "", "image/png", img_byte_arr 
    return "ERR_AIWORK_FAILURE", "", []

def repacktask(inputId, mime, data, task, args):
    r = df_pb2.Output()
    r.InputId = inputId
    r.Version = 1
    r.ProducerName = cfg["WORKER"]["NAME"]
    r.ProducerSign = "mysig_taskid_userid"
    r.MimeType = mime
    r.Data = data
    r.Args = args

    task.OutputList.append(r)

    nexttask = task.InputList[len(task.OutputList)]
    output = task.SerializeToString()
    return output, nexttask

cfg = loadconf()
pipe = loadmodelpipe()

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

