#!/usr/bin/env python3
"""Surya layout detection entrypoint.

Takes a page image path and outputs JSON with detected text regions
and their bounding boxes.

Usage: python entrypoint.py /input/page.png
Output: JSON to stdout with {"regions": [{"bbox": [x,y,w,h], "text": "..."}]}
"""

import json
import sys

from PIL import Image
from surya.detection import batch_text_detection
from surya.model.detection.model import load_model as load_det_model
from surya.model.detection.processor import load_processor as load_det_processor
from surya.ocr import run_ocr
from surya.model.recognition.model import load_model as load_rec_model
from surya.model.recognition.processor import load_processor as load_rec_processor


def main():
    if len(sys.argv) < 2:
        print("Usage: entrypoint.py <image_path>", file=sys.stderr)
        sys.exit(1)

    image_path = sys.argv[1]
    image = Image.open(image_path)

    # Load models
    det_model = load_det_model()
    det_processor = load_det_processor()
    rec_model = load_rec_model()
    rec_processor = load_rec_processor()

    # Run OCR with layout detection
    langs = ["ar"]  # Arabic primary
    results = run_ocr(
        [image],
        [langs],
        det_model,
        det_processor,
        rec_model,
        rec_processor,
    )

    regions = []
    if results:
        for line in results[0].text_lines:
            bbox = line.bbox  # [x1, y1, x2, y2]
            # Convert to [x, y, width, height]
            x, y = int(bbox[0]), int(bbox[1])
            w, h = int(bbox[2] - bbox[0]), int(bbox[3] - bbox[1])
            regions.append({
                "bbox": [x, y, w, h],
                "text": line.text,
            })

    json.dump({"regions": regions}, sys.stdout, ensure_ascii=False)


if __name__ == "__main__":
    main()
