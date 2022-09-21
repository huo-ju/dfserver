import os, sys, io, random, ssl, time, json, random
import logging, configparser
import threading
from threading import Lock
import pika
#from PIL import Image
import cv2
import numpy as np
from basicsr.archs.rrdbnet_arch import RRDBNet
import df_pb2


root_path = os.getcwd()
sys.path.append(f'{root_path}/Real-ESRGAN')

from realesrgan import RealESRGANer
from realesrgan.archs.srvgg_arch import SRVGGNetCompact

tasklock = Lock()

logging.basicConfig(
    format="%(asctime)s %(message)s",
    stream=sys.stdout,
    level=logging.WARNING,
    filemode="w",
)

def loadmodel():
    modelname='RealESRGAN_x4plus'
    model = RRDBNet(num_in_ch=3, num_out_ch=3, num_feat=64, num_block=23, num_grow_ch=32, scale=4)
    netscale = 4
    # determine model paths
    model_path = os.path.join('Real-ESRGAN/experiments/pretrained_models', modelname + '.pth')
    if not os.path.isfile(model_path):
        raise ValueError(f'Model {modelname} does not exist.')
    
    config = {"tile":0, "tile_pad":10, "pre_pad":0}
    
    upsampler = RealESRGANer(
        scale=netscale,
        model_path=model_path,
        model=model,
        tile=config["tile"],
        tile_pad=config["tile_pad"],
        pre_pad=config["pre_pad"],
        half=True,
        gpu_id=0)
    return upsampler


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
            #print(task)

            err, inputId, mime, data, args = taskRunner(task)
            if err == "":
                print("upscaler task Succ, send ack.", inputId, task.TaskId)
                pbdata, nexttask = repacktask(inputId, mime,  data, task, args)
                #find next inputtask
                r = publish(pbdata, channel, nexttask.Name)
                if r == "":
                    logging.debug("upscaler task Succ, send ack.")
                    channel.basic_ack(method.delivery_tag)
                else:
                    logging.debug("upscaler task Err:", r)
                    channel.basic_ack(method.delivery_tag)
            else:
                print("upscaler task failure, send nack.", inputId, task.TaskId)
                print(err)
                logging.debug("upscaler task failure, send nack, err:", err)
                channel.basic_ack(method.delivery_tag)

    def run(self):
        print("start thread:", self.tid)
        self.channel.start_consuming()

def loadconf():
    config = configparser.ConfigParser()
    config.read("configs/upscaleconfig.ini")
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
    if ainame == 'ai.realesrgan':
        inputsettings = json.loads(inputtask.Settings)
        data = None
        if prevoutput != None:
            data = prevoutput.Data
        elif inputtask.Data != None:
            data = inputtask.Data
        if data == None:
            return "ERR_NO_INPUT_IMAGE_DATA" , "", "" ,[] ,""
        r, mime, data, finalsettings = realesrganwork(data, inputsettings)
        print("image len:", len(data))
        #build output object and update the task
        args = ""
        for x, y in finalsettings.items():
            args = args + "{}:{} ".format(x, y)
        return r, inputtask.InputId, mime, data, bytes(args, 'utf-8')
    else: 
        #not success, return errr and nack
        return "ERR_NOT_SUPPORT_MODEL" , "", "" ,[] ,""
    

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
        
def realesrganwork(inputdata, inputsettings):
    #TODO: apply user inputsettings instead the default settings
    settings = inputsettings
    try:
        nparr = np.frombuffer(inputdata, np.uint8)
        image = cv2.imdecode(nparr, cv2.IMREAD_UNCHANGED) 
        output, _ = upsampler.enhance(image, outscale=4)
        #save_path = os.path.join(f'output4x.png')
        img_byte_arr= cv2.imencode('.png', output)[1].tobytes()
        return "", "image/png", img_byte_arr , settings
    except Exception as e:
        print("ERR_UPSCALE_FAILURE")
        print(e)
        return "ERR_UPSCALE_FAILURE", "", [], {}

def repacktask(inputId, mime, data, task, args):
    r = df_pb2.Output()
    r.InputId = inputId
    r.Version = 1
    r.ProducerName = cfg["WORKER"]["NAME"]
    r.ProducerSign = "mysig_taskid_userid"
    r.MimeType = mime
    r.Data = data
    r.Args = args

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
upsampler = loadmodel()

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

