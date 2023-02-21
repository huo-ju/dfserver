import os
import sys
import io
import logging
root_path = os.getcwd()
sys.path.append(f"{root_path}/diffusers/src")

import diffusers
from diffusers import StableDiffusionPipeline, LMSDiscreteScheduler
from extra.t2iadapter.adapter import Adapter
import json
import random
from torch import autocast
import torch
from patch.dummychecker import DummySafetyChecker  # , DummyFeatureExtractor
import patch.dummychecker as dummychecker
import promptutils

logging.basicConfig(
    format="%(asctime)s %(message)s",
    stream=sys.stdout,
    level=logging.WARNING,
    filemode="w",
)

print("version:")
print("diffusers:", diffusers.__version__)


class Worker:
    def __init__(self, device, args):
        self.device = device
        self.args = args
        self.model_name = ""

    def loadmodel(self):
        config = {}
        if "config" in self.args:
            config = self.args["config"]

        lms = LMSDiscreteScheduler(
            beta_start=0.00085, beta_end=0.012, beta_schedule="scaled_linear"
        )

        usefp16 = False
        usesafetychecker = True
        model_id = "CompVis/stable-diffusion-v1-4"

        if "USE_FP16" in config:
            if config["USE_FP16"].lower() == "true":
                usefp16 = True

        if "USE_SAFETYCHECKER" in config:
            if config["USE_SAFETYCHECKER"].lower() == "false":
                usesafetychecker = False

        if "MODEL_ID" in config:
            model_id = config["MODEL_ID"]

        if "MODEL_NAME" in config:
            self.model_name = config["MODEL_NAME"]


        #model_id = "hakurei/waifu-diffusion"
        print("model_id", model_id)
        print("model_name", self.model_name)

        safechecker = DummySafetyChecker().safety_checker
        if usesafetychecker == True:
            from diffusers.pipelines.stable_diffusion import (
                StableDiffusionSafetyChecker,
            )

            safechecker = StableDiffusionSafetyChecker.from_pretrained(
                "CompVis/stable-diffusion-safety-checker"
            )

        if usefp16 == True:
            print("load float16 model")
            self.pipe = StableDiffusionPipeline.from_pretrained(
                model_id,
                revision="fp16",
                torch_dtype=torch.float16,
                scheduler=lms,
                safety_checker=safechecker,
                use_auth_token=True,
            ).to("cuda")
        else:
            print("load float32 model")
            self.pipe = StableDiffusionPipeline.from_pretrained(
                model_id, scheduler=lms, safety_checker=safechecker, use_auth_token=True
            ).to("cuda")


        if "ADAPTER_MODEL" in config:
            self.adaptermodel = {}
            device = "cuda"
            for key in config["ADAPTER_MODEL"]:
                model_ad = None
                if key == "sketch":
                    model_ad = Adapter(channels=[320, 640, 1280, 1280][:4], nums_rb=2, ksize=1, sk=True, use_conv=False).to(device)
                elif key == "keypose":
                    model_ad = Adapter(cin=int(3*64), channels=[320, 640, 1280, 1280][:4], nums_rb=2, ksize=1, sk=True, use_conv=False).to(device)
                elif key == "seg":
                    model_ad = Adapter(cin=int(3*64), channels=[320, 640, 1280, 1280][:4], nums_rb=2, ksize=1, sk=True, use_conv=False).to(device)
                if model_ad is None:
                    print("can't load unknown adapter model:" + key)
                    continue
                if usefp16 == True:
                    model_ad.half()
                admodelpath = config["ADAPTER_MODEL"][key]
                model_ad.load_state_dict(torch.load(admodelpath))
                self.adaptermodel[key] = model_ad

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
            segs = promptutils.promptToSegs(inputsettings["prompt"])
            for seg in segs:
                print(seg)
                if seg["weight"]<0:
                    if "negative_prompt" not in settings:
                        settings["negative_prompt"] = seg["seg"]
                    else:
                        settings["negative_prompt"] += " " + seg["seg"] 
                else: 
                    if "prompt" not in settings:
                        settings["prompt"] = seg["seg"]
                    else:
                        settings["prompt"] += " " + seg["seg"]

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

        customgenerator = torch.Generator(device="cuda")
        customgenerator = customgenerator.manual_seed(inputsettings["seed"])
        settings["generator"] = customgenerator

        logging.debug("aiwork with settings:")
        logging.debug(settings)
        try:
            with autocast("cuda"):
                image = self.pipe(**settings).images[0]
            img_byte_arr = io.BytesIO()
            image.save(img_byte_arr, format="PNG")
            img_byte_arr = img_byte_arr.getvalue()

            settings.pop("generator", None)
            settings["seed"] = inputsettings["seed"]
            return "", "image/png", img_byte_arr, settings
        except Exception as e:
            print(e)
            return "ERR_AIWORK_FAILURE", "", [], {}

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

        if self.model_name != "":
            output = output + " -M " + self.model_name

        return output
