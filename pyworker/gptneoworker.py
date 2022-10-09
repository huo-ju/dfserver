import sys, logging
import json
from transformers import GPTNeoForCausalLM, AutoTokenizer

logging.basicConfig(
    format="%(asctime)s %(message)s",
    stream=sys.stdout,
    level=logging.WARNING,
    filemode="w",
)

def gptoutputToPrompt(output):
    outputtag = "<|modeloutput|>"
    i = output.find(outputtag)
    output= output[i+len(outputtag) :len(output)]
    i = output.find(outputtag)
    if i > 0:
        output = output[0:i]
    return output 

class Worker:
    def __init__(self, device, args):
        self.device = device
        self.args = args

    def loadmodel(self):
        config = {}
        if "config" in self.args:
            config = self.args["config"]

        model_id = "huoju/gptneoforsdprompt"
        self.model = GPTNeoForCausalLM.from_pretrained(model_id).half().to("cuda")
        self.tokenizer = AutoTokenizer.from_pretrained(model_id)
        

    def work(self, inputtask, prevoutput):
        inputsettings = json.loads(inputtask.Settings)
        if "prompt" in inputsettings:
            if len(inputsettings["prompt"]) == 0:
                return "ERR_NO_INPUT_PROMPT", "", {}, {}
            prompt = inputsettings["prompt"]

            if prompt.endswith("..."):
                prompt = prompt[0:len(prompt)-3]
                prompt = prompt.strip()
                prompt = "<|userinput|>" + prompt.strip()
            else:
                prompt = "<|userinput|>" + prompt.strip() + "<|modeloutput|>"

            ids = self.tokenizer(prompt, return_tensors="pt").input_ids.to("cuda")
            max_length = 100 + ids.shape[1]
            if max_length > 200:
               max_length = 200

            gen_tokens = self.model.generate(
                ids,
                do_sample=True,
                min_length=max_length,
                max_length=max_length,
                temperature=0.9,
                use_cache=True
            )
            gen_text = self.tokenizer.batch_decode(gen_tokens)[0]
            gptoutputdata = gptoutputToPrompt(gen_text)
            data = str.encode(gptoutputdata, "utf-8")
            return "", "text/plain", data, inputsettings
        else:
            return "ERR_NO_INPUT_PROMPT", "", {}, {}

        print(inputsettings)

    def settingsToOutput(self, settings, finalsettings):
        return "prompt: " + settings["prompt"]
