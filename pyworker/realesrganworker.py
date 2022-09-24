import os, sys, io, logging
import json
import numpy as np
import cv2

from basicsr.archs.rrdbnet_arch import RRDBNet

root_path = os.getcwd()
sys.path.append(f"{root_path}/Real-ESRGAN")
from realesrgan import RealESRGANer
from realesrgan.archs.srvgg_arch import SRVGGNetCompact

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
        modelname = "RealESRGAN_x4plus"
        model = RRDBNet(
            num_in_ch=3,
            num_out_ch=3,
            num_feat=64,
            num_block=23,
            num_grow_ch=32,
            scale=4,
        )
        netscale = 4
        # determine model paths
        model_path = os.path.join(
            "Real-ESRGAN/experiments/pretrained_models", modelname + ".pth"
        )
        if not os.path.isfile(model_path):
            raise ValueError(f"Model {args.model_name} does not exist.")

        config = {"tile": 0, "tile_pad": 10, "pre_pad": 0}

        self.upsampler = RealESRGANer(
            scale=netscale,
            model_path=model_path,
            model=model,
            tile=config["tile"],
            tile_pad=config["tile_pad"],
            pre_pad=config["pre_pad"],
            half=True,
            gpu_id=0,
        )

    def work(self, inputtask, prevoutput):
        # TODO: apply user inputsettings instead the default settings
        inputsettings = json.loads(inputtask.Settings)
        settings = inputsettings

        inputsettings = {}

        data = None
        if prevoutput != None:
            data = prevoutput.Data
        elif inputtask.Data != None:
            data = inputtask.Data
        if data == None:
            return "ERR_NO_INPUT_IMAGE_DATA}", "",bytes() , {}

        try:
            nparr = np.frombuffer(data, np.uint8)
            image = cv2.imdecode(nparr, cv2.IMREAD_UNCHANGED)
            output, _ = self.upsampler.enhance(image, outscale=4)
            img_byte_arr = cv2.imencode(".png", output)[1].tobytes()
            return "", "image/png", img_byte_arr, settings
        except Exception as e:
            print("ERR_UPSCALE_FAILURE")
            print(e)
            return "ERR_UPSCALE_FAILURE: {}".format(e), "", bytes(), {}

    def settingsToOutput(self, settings, finalsettings):
        return ""
