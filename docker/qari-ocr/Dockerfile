FROM python:3.11-slim

RUN pip install --no-cache-dir \
    torch \
    transformers \
    accelerate \
    qwen_vl_utils \
    flask \
    Pillow

# Download model weights at build time — no runtime downloads
# v0.3: full merged model, has model.safetensors, loads directly
RUN python -c "from transformers import Qwen2VLForConditionalGeneration, AutoProcessor; Qwen2VLForConditionalGeneration.from_pretrained('NAMAA-Space/Qari-OCR-v0.3-VL-2B-Instruct'); AutoProcessor.from_pretrained('NAMAA-Space/Qari-OCR-v0.3-VL-2B-Instruct')"

COPY server.py /app/server.py
WORKDIR /app
EXPOSE 8000
ENTRYPOINT ["python", "server.py"]
