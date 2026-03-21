FROM python:3.11-slim

# Install system dependencies required by OpenCV
RUN apt-get update && apt-get install -y --no-install-recommends \
    libgl1 libglib2.0-0 libxcb1 libsm6 libxext6 libxrender1 \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir doclayout-yolo huggingface-hub

# Download model at build time so it's cached in the image
RUN python -c "from huggingface_hub import hf_hub_download; \
    hf_hub_download(repo_id='juliozhao/DocLayout-YOLO-DocStructBench', \
    filename='doclayout_yolo_docstructbench_imgsz1024.pt', \
    local_dir='/models')"

COPY entrypoint.py /app/entrypoint.py
WORKDIR /app
ENTRYPOINT ["python", "entrypoint.py"]
