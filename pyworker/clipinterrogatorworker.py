import os, sys, io, logging
import torch
from torchvision import transforms
from torchvision.transforms.functional import InterpolationMode
from PIL import Image

root_path = os.getcwd()
sys.path.append(f'{root_path}/CLIP')
from CLIP import clip

sys.path.append(f'{root_path}/BLIP')
from models.blip import blip_decoder

logging.basicConfig(
    format="%(asctime)s %(message)s",
    stream=sys.stdout,
    level=logging.WARNING,
    filemode="w",
)

models = ['ViT-L/14']
blip_image_eval_size = 384

def load_list(filename):
    with open(filename, 'r', encoding='utf-8', errors='replace') as f:
        items = [line.strip() for line in f.readlines()]
    return items

class Worker:
    def __init__(self, deviceid, args):
        self.device = torch.device(deviceid)
        data_path = f'{root_path}/data/'
        self.artists = load_list(os.path.join(data_path, 'artists.txt'))
        self.flavors = load_list(os.path.join(data_path, 'flavors.txt'))
        self.mediums = load_list(os.path.join(data_path, 'mediums.txt'))
        self.movements = load_list(os.path.join(data_path, 'movements.txt'))
        
        sites = ['Artstation', 'behance', 'cg society', 'cgsociety', 'deviantart', 'dribble', 'flickr', 'instagram', 'pexels', 'pinterest', 'pixabay', 'pixiv', 'polycount', 'reddit', 'shutterstock', 'tumblr', 'unsplash', 'zbrush central']
        self.trending_list = [site for site in sites]
        self.trending_list.extend(["trending on "+site for site in sites])
        self.trending_list.extend(["featured on "+site for site in sites])
        self.trending_list.extend([site+" contest winner" for site in sites])

    def loadmodel(self):
        self.clipmodels = {}
        for model_name in models:
            print("load clip model:", model_name)
            model, preprocess = clip.load(model_name, device=self.device, jit=True)
            model = model.to(self.device)
            model.eval()
            self.clipmodels[model_name] = {"model": model,"preprocess": preprocess}
            print("model loaded: ", model_name)

        print("load blip model")
        blip_model_url = f'{root_path}/models/model_base_caption.pth'        
        blip_model = blip_decoder(pretrained=blip_model_url, med_config=f'{root_path}/BLIP/configs/med_config.json',image_size=blip_image_eval_size, vit='base')
        self.blip_model = blip_model.to(self.device)
        self.blip_model.eval()

    def work(self, inputtask, prevoutput):
        inputsettings = {}
        data = None
        if prevoutput != None:
            data = prevoutput.Data
        elif inputtask.Data != None:
            data = inputtask.Data 
        if data == None:
            return "ERR_NO_INPUT_IMAGE_DATA", "", {}, {}

        dataStream = io.BytesIO(data)
        pilimage = Image.open(dataStream).convert('RGB')
        outputdesc = self.interrogate('ViT-L/14',pilimage)
        args = ""
        data = str.encode(outputdesc, "utf-8")
        return "", "text/plain", data, inputsettings

    def generate_caption(self, image):
        gpu_image = transforms.Compose([
            transforms.Resize((blip_image_eval_size, blip_image_eval_size), interpolation=InterpolationMode.BICUBIC),
            transforms.ToTensor(),
            transforms.Normalize((0.48145466, 0.4578275, 0.40821073), (0.26862954, 0.26130258, 0.27577711))
        ])(image).unsqueeze(0).to(self.device)
    
        with torch.no_grad():
            caption = self.blip_model.generate(gpu_image, sample=False, num_beams=3, max_length=20, min_length=5)
        return caption[0]

    def interrogate(self, modelname, image):
        caption = self.generate_caption(image)
        if len(models) == 0:
            print(f"\n\n{caption}")
            return
    
        table = []
        bests = [[('',0)]]*5
        clipmodel = self.clipmodels[modelname]
    
        preprocess = clipmodel["preprocess"]
        model = clipmodel["model"]
    
        images = preprocess(image).unsqueeze(0).cuda()
        with torch.no_grad():
            image_features = model.encode_image(images).float()
        image_features /= image_features.norm(dim=-1, keepdim=True)
    
        ranks = [
            self.rank(model, image_features, self.mediums),
            self.rank(model, image_features, ["by "+artist for artist in self.artists]),
            self.rank(model, image_features, self.trending_list),
            self.rank(model, image_features, self.movements),
            self.rank(model, image_features, self.flavors, top_count=3)
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


    def rank(self, model, image_features, text_array, top_count=1):
        top_count = min(top_count, len(text_array))
        text_tokens = clip.tokenize([text for text in text_array]).cuda()
        with torch.no_grad():
            text_features = model.encode_text(text_tokens).float()
        text_features /= text_features.norm(dim=-1, keepdim=True)
    
        similarity = torch.zeros((1, len(text_array))).to(self.device)
        for i in range(image_features.shape[0]):
            similarity += (100.0 * image_features[i].unsqueeze(0) @ text_features.T).softmax(dim=-1)
        similarity /= image_features.shape[0]
    
        top_probs, top_labels = similarity.cpu().topk(top_count, dim=-1)  
        return [(text_array[top_labels[0][i].numpy()], (top_probs[0][i].numpy()*100)) for i in range(top_count)]
    
    def settingsToOutput(self, settings, finalsettings):
        return ""
