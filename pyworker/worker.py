import os, sys, io, random, ssl, time, json
import importlib
import logging, configparser
import threading
from threading import Lock
import pika
from PIL import Image
import df_pb2

tasklock = Lock()

logging.basicConfig(
    format="%(asctime)s %(message)s",
    stream=sys.stdout,
    level=logging.WARNING,
    filemode="w",
)


class ThreadedConsumer(threading.Thread):
    def __init__(self, tid, taskqueuename, config):
        threading.Thread.__init__(self)
        self.tid = tid
        taskqueuekey = tid
        amqpurl = config["AMQP_URL"]
        parameters = pika.URLParameters(amqpurl)

        if "TLS_CA_CERT" in config:
            context = ssl.create_default_context(cafile=config["TLS_CA_CERT"])
            context.load_cert_chain(config["TLS_CLIENT_CERT"], config["TLS_CLIENT_KEY"])
            ssl_options = pika.SSLOptions(context, "localhost")
            parameters.ssl_options = ssl_options

        self.connection = pika.BlockingConnection(parameters)
        self.channel = self.connection.channel()
        self.channel.basic_qos(prefetch_count=1)
        qname = taskqueuename + "." + taskqueuekey
        print("waiting tasks from {}".format(qname))
        self.channel.basic_consume(
            queue=qname, on_message_callback=self.callback, auto_ack=False
        )
        threading.Thread(
            target=self.channel.basic_consume(
                queue=qname, on_message_callback=self.callback, auto_ack=False
            )
        )

    def callback(self, channel, method, properties, body):
        print(
            "[x] Received from {}, body length {}".format(method.routing_key, len(body))
        )
        with tasklock:
            print(" [*] task start...")

            task = df_pb2.Task()
            task.ParseFromString(body)

            err, inputId, mime, data, args = taskRunner(task)
            print("AI task Succ, send ack.", inputId, task.TaskId)
            pbdata, nexttask = repacktask(inputId, mime, data, task, args, err)
            # find next inputtask
            r = publish(pbdata, channel, nexttask.Name)
            if r == "":
                logging.debug("AI task Succ, send ack.")
            else:
                logging.debug("AI task Err:", r)
            channel.basic_ack(method.delivery_tag)

    def run(self):
        print("start thread:", self.tid)
        self.channel.start_consuming()


def loadconf(loadworkername):
    config = configparser.ConfigParser()
    config.optionxform = str
    configfilename = "configs/{}_config.ini".format(loadworkername)
    print("loading config:", configfilename)
    config.read_file(open(configfilename, "r"))
    return config


def taskRunner(task):
    outputlen = len(task.OutputList)
    if outputlen >= len(task.InputList):
        return

    inputtask = task.InputList[outputlen]
    prevoutput = None
    if outputlen > 0:
        prevoutput = task.OutputList[outputlen - 1]

    ainame = inputtask.Name
    if ainame == workername:
        try:
            inputsettings = json.loads(inputtask.Settings)
            r, mime, data, finalsettings = worker.work(inputtask, prevoutput)
            print("image len:", len(data))
            args = worker.settingsToOutput(inputsettings, finalsettings)
            return r, inputtask.InputId, mime, data, bytes(args, "utf-8")
        except Exception as e:
            return "error: {}".format(e), "", "", [], ""
    else:
        # not success, return errr and nack
        return "ERR_NOT_SUPPORT_MODEL", "", "", [], ""


def publish(result, channel, taskname):
    l = taskname.split(".")
    if len(l) == 3:  # with route key
        key = l[2]
        exchange = l[0] + "." + l[1]
        channel.basic_publish(exchange=exchange, routing_key=key, body=result)
        return ""
    elif len(l) == 2:  # no special routekey, use all as route key
        exchange = l[0] + "." + l[1]
        channel.basic_publish(exchange=exchange, routing_key="all", body=result)
        return ""
    else:
        return "ERR_INVAILD_TASKNAME"

def repacktask(inputId, mime, data, task, args, error):
    r = df_pb2.Output()
    r.InputId = inputId
    r.Version = 1
    r.ProducerName = cfg["WORKER"]["NAME"]
    r.ProducerSign = "mysig_taskid_userid"
    if error == "":
        r.MimeType = mime
        r.Data = data
        r.Args = args
    else:
        r.MimeType = "text/plain"
        r.Error = error

    task.OutputList.append(r)

    nexttask = task.InputList[len(task.OutputList)]
    output = task.SerializeToString()
    return output, nexttask

if __name__ == "__main__":
    global workername, cfg, worker
    loadworkername = "sd14"
    if len(sys.argv) > 1:
        loadworkername = sys.argv[1]

    print("load worker :", loadworkername)
    cfg = loadconf(loadworkername)

    workermodule = importlib.import_module(loadworkername + "worker")
    workcfg = {}
    if "MODEL" in cfg:
        modelcfg = cfg["MODEL"]
        workcfg.update(modelcfg)
    if "SETTING" in cfg:
        settingscfg = cfg["SETTING"]
        workcfg.update(settingscfg)
    if "ADAPTER_MODEL" in cfg:
        modelcfg = cfg["ADAPTER_MODEL"]
        workcfg["ADAPTER_MODEL"] = modelcfg

    worker = workermodule.Worker("cuda:0", {"config": workcfg})
    worker.loadmodel()
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

