# Using a GPU for the local AI

NovelCheck talks to AI servers with the standard OpenAI-style chat protocol, so any server that speaks it works:
Ollama, or llama.cpp's own `llama-server`. The GPU belongs to that server, not to NovelCheck.

## Intel Arc cards, as a separate app

Ollama reaches Intel GPUs through its Vulkan backend, which is still experimental and can crash on some cards
(`llama-server process has terminated: signal: segmentation fault`, or a `vk::PhysicalDevice::createDevice` error).
llama.cpp's own server can be tried on the same GPU:

```yaml
services:
  llama:
    image: ghcr.io/ggml-org/llama.cpp:server-vulkan   # or server-intel (SYCL)
    restart: unless-stopped
    ports: ["8080:8080"]
    devices: ["/dev/dri:/dev/dri"]
    volumes: ["/path/to/models:/root/.cache/llama.cpp"]
    command: ["-hf", "bartowski/Qwen2.5-7B-Instruct-GGUF:Q4_K_M", "-ngl", "99", "-c", "8192", "--host", "0.0.0.0", "--port", "8080"]
```

Check its log for the GPU's name and that all layers are offloaded. Then add it in **Admin → AI** with the address
`http://<host>:8080/v1`, no key, and any model name, and compare it with the benchmark button.

## What fits

A 6 GB card holds a 3B to 4B model easily and a 7B to 8B model at 4-bit (about 4.7 GB) with little room for long text.
Larger models need a bigger card. Only one model server should keep a model loaded on the card at a time.
