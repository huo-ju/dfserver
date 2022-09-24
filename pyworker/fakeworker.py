import os, sys, io, logging

root_path = os.getcwd()
import json
import random


logging.basicConfig(
    format="%(asctime)s %(message)s",
    stream=sys.stdout,
    level=logging.WARNING,
    filemode="w",
)


class Worker:
    def __init__(self, device, args):
        self.device = device

    def loadmodel(self):
        print("loading fake model...")

    def work(self, inputtask, prevoutput):
        inputsettings = json.loads(inputtask.Settings)
        settings = {
            "height": 512,
            "width": 512,
            "num_inference_steps": 50,
            "guidance_scale": 7.5,
            "eta": 0,
        }
        if inputsettings["prompt"] != "":
            settings["prompt"] = inputsettings["prompt"]
        if inputsettings["height"] > 0:
            settings["height"] = inputsettings["height"]
        if inputsettings["width"] > 0:
            settings["width"] = inputsettings["width"]
        if inputsettings["num_inference_steps"] > 0:
            settings["num_inference_steps"] = inputsettings["num_inference_steps"]
        if inputsettings["guidance_scale"] > 0:
            settings["guidance_scale"] = inputsettings["guidance_scale"]
        if inputsettings["eta"] > 0:
            settings["eta"] = inputsettings["eta"]

        if "seed" not in inputsettings or inputsettings["seed"] == 0:
            inputsettings["seed"] = random.randint(1000000000, 9999999999)


        logging.debug("aiwork with settings:")
        logging.debug(settings)

        try:
            #raise Exception("a test message") #test error message response
            with open("output.png", "rb") as fh:
                buf = io.BytesIO(fh.read())
                settings["seed"] = inputsettings["seed"]
                return "", "image/png", buf.getvalue(), settings
        except Exception as e:
            print(e)
            return "ERR_AIWORK_FAILURE: {}".format(e), "",bytes() , {}

    def settingsToOutput(self, settings, finalsettings):
        output = settings["prompt"]
        if settings["height"] != 0:
            output = output + " -H " + str(settings["height"])
        if settings["width"] != 0:
            output = output + " -W " + str(settings["width"])
        if settings["guidance_scale"] != 0:
            output = output + " -C " + str(settings["guidance_scale"])
        if settings["num_inference_steps"] != 0:
            output = output + " -s " + str(settings["num_inference_steps"])
        if "seed" in settings and settings["seed"] != 0:
            output = output + " -S " + str(settings["seed"])
        elif "seed" in finalsettings and finalsettings["seed"] != 0:
            output = output + " -S " + str(finalsettings["seed"])
        return output
