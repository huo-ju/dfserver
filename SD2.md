Stable Diffusion 2 support

*** this is an experimental version ***

### dfserver Configuration

Edit `config.toml` of dferver, Add a new bindkey to the `sd14` qeueue: 

Example: `wf` for waifu diffusion, `sd2` for Stable Diffusion 2, `all` for Stable Diffusion 1.4

```
[QUEUE."ai.sd14"]
name = "ai.sd14"
bindkeys = ["all","wf", "sd2"]
```

Then, ***Restart dfserver***

### SD2 Worker Install 

Install the main branch of diffusers

```bash
pip install --upgrade git+https://github.com/huggingface/diffusers.git transformers accelerate scipy
```

Copy sd20worker.py and config to the GPU server

```bash
cp configs/sd20_config.ini.sample configs/sd20mega_config.ini 
```

### Run the sd20 ai worker
```bash
python worker.py sd20
```

### Using

!dream a cat -M sd2```
