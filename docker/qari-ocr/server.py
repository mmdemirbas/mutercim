"""Qari-OCR HTTP server — persistent Flask server that loads the model once at startup."""

import os
import time
import json
import logging
import threading

from flask import Flask, request, jsonify
from PIL import Image
import torch
from transformers import Qwen2VLForConditionalGeneration, AutoProcessor
from qwen_vl_utils import process_vision_info

app = Flask(__name__)
logger = logging.getLogger("qari-ocr")
logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")

MODEL_NAME = os.environ.get("MODEL", "NAMAA-Space/Qari-OCR-0.2.2.1-VL-2B-Instruct")
QUANTIZE = os.environ.get("QUANTIZE", "8bit")
DEVICE = os.environ.get("DEVICE", "cpu")
MAX_TOKENS = int(os.environ.get("MAX_TOKENS", "2000"))

model = None
processor = None
model_ready = False

DEFAULT_PROMPT = (
    "Below is the image of a section from an Arabic document. "
    "Return the plain text exactly as written, preserving all diacritical marks (tashkeel). "
    "Do not hallucinate or add text that is not in the image."
)


def load_model():
    """Load the Qari-OCR model and processor."""
    global model, processor, model_ready
    logger.info(f"loading model {MODEL_NAME} (quantize={QUANTIZE}, device={DEVICE})")
    start = time.time()

    load_kwargs = {"torch_dtype": "auto", "device_map": DEVICE}
    if QUANTIZE == "8bit":
        from transformers import BitsAndBytesConfig
        load_kwargs["quantization_config"] = BitsAndBytesConfig(load_in_8bit=True)

    model = Qwen2VLForConditionalGeneration.from_pretrained(MODEL_NAME, **load_kwargs)
    processor = AutoProcessor.from_pretrained(MODEL_NAME)
    model_ready = True
    logger.info(f"model loaded in {time.time() - start:.1f}s")


def run_ocr(image, prompt=None):
    """Run OCR on a PIL Image. Returns (text, elapsed_ms)."""
    if prompt is None:
        prompt = DEFAULT_PROMPT

    # Save to temp file for qwen_vl_utils
    tmp_path = "/tmp/qari_input.png"
    image.save(tmp_path)

    messages = [
        {"role": "user", "content": [
            {"type": "image", "image": f"file://{tmp_path}"},
            {"type": "text", "text": prompt},
        ]}
    ]

    text = processor.apply_chat_template(messages, tokenize=False, add_generation_prompt=True)
    image_inputs, video_inputs = process_vision_info(messages)
    inputs = processor(text=[text], images=image_inputs, videos=video_inputs,
                       padding=True, return_tensors="pt")
    inputs = inputs.to(model.device)

    start = time.time()
    with torch.no_grad():
        generated_ids = model.generate(**inputs, max_new_tokens=MAX_TOKENS)
    elapsed_ms = int((time.time() - start) * 1000)

    trimmed = [out[len(inp):] for inp, out in zip(inputs.input_ids, generated_ids)]
    result = processor.batch_decode(trimmed, skip_special_tokens=True,
                                    clean_up_tokenization_spaces=False)[0]

    os.remove(tmp_path)
    return result.strip(), elapsed_ms


@app.route("/health")
def health():
    """Health check endpoint."""
    if model_ready:
        return jsonify({"status": "ready", "model": MODEL_NAME, "quantize": QUANTIZE})
    return jsonify({"status": "loading"}), 503


@app.route("/ocr", methods=["POST"])
def ocr_single():
    """OCR a single image, returns full page text."""
    if not model_ready:
        return jsonify({"error": "model not loaded"}), 503

    image_file = request.files.get("image")
    if not image_file:
        return jsonify({"error": "no image provided"}), 400

    prompt = request.form.get("prompt", None)
    image = Image.open(image_file.stream).convert("RGB")
    text, elapsed_ms = run_ocr(image, prompt)

    return jsonify({
        "text": text,
        "model": MODEL_NAME,
        "elapsed_ms": elapsed_ms,
    })


@app.route("/ocr/regions", methods=["POST"])
def ocr_regions():
    """Crop and OCR multiple regions from one page image."""
    if not model_ready:
        return jsonify({"error": "model not loaded"}), 503

    image_file = request.files.get("image")
    regions_json = request.form.get("regions")
    if not image_file or not regions_json:
        return jsonify({"error": "image and regions required"}), 400

    prompt = request.form.get("prompt", None)
    image = Image.open(image_file.stream).convert("RGB")
    regions = json.loads(regions_json)

    results = []
    total_start = time.time()

    for region in regions:
        rid = region["id"]
        bbox = region["bbox"]  # [x1, y1, x2, y2]
        # Add 3px padding, clamp to image bounds
        pad = 3
        x1 = max(0, bbox[0] - pad)
        y1 = max(0, bbox[1] - pad)
        x2 = min(image.width, bbox[2] + pad)
        y2 = min(image.height, bbox[3] + pad)
        crop = image.crop((x1, y1, x2, y2))

        text, elapsed_ms = run_ocr(crop, prompt)
        results.append({"id": rid, "text": text, "elapsed_ms": elapsed_ms})
        logger.info(f"region {rid}: {elapsed_ms}ms, {len(text)} chars")

    total_ms = int((time.time() - total_start) * 1000)
    return jsonify({
        "results": results,
        "model": MODEL_NAME,
        "total_elapsed_ms": total_ms,
    })


if __name__ == "__main__":
    # Load model in background so health endpoint is available immediately
    threading.Thread(target=load_model, daemon=True).start()

    port = int(os.environ.get("PORT", "8000"))
    logger.info(f"starting server on port {port}")
    # threaded=False: model is not thread-safe, one request at a time
    app.run(host="0.0.0.0", port=port, threaded=False)
