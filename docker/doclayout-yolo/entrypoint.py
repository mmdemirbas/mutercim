#!/usr/bin/env python3
"""DocLayout-YOLO layout detection entrypoint.

Takes a page image path (or directory of images) and outputs JSON with
detected document regions, their bounding boxes, types, and confidence scores.

Usage:
    Single file:  python entrypoint.py /input/page.png
    Directory:    python entrypoint.py /input/

Output (single file): JSON to stdout:
    {"regions": [{"bbox": [x1, y1, x2, y2], "type": "title", "confidence": 0.95}],
     "image_size": {"width": 1000, "height": 1500}}

Output (directory): JSONL to stdout, one JSON per line:
    {"file": "001.png", "regions": [...], "image_size": {...}}

Environment:
    DEVICE  - torch device (default: "cpu"). Set to "cuda" for GPU.
    MODEL   - model path (default: /models/doclayout_yolo_docstructbench_imgsz1024.pt)
"""

import json
import os
import sys
from pathlib import Path

from doclayout_yolo import YOLOv10


def load_model():
    """Load the DocLayout-YOLO model."""
    model_path = os.environ.get(
        "MODEL", "/models/doclayout_yolo_docstructbench_imgsz1024.pt"
    )
    return YOLOv10(model_path)


def detect_regions(model, image_path, device):
    """Run detection on a single image and return result dict."""
    results = model.predict(
        str(image_path), imgsz=1024, conf=0.2, device=device, verbose=False
    )

    regions = []
    if results and len(results) > 0:
        result = results[0]
        boxes = result.boxes
        for i in range(len(boxes)):
            xyxy = boxes.xyxy[i].tolist()  # [x1, y1, x2, y2]
            conf = float(boxes.conf[i])
            cls_id = int(boxes.cls[i])
            cls_name = result.names[cls_id]

            regions.append({
                "bbox": [int(xyxy[0]), int(xyxy[1]), int(xyxy[2]), int(xyxy[3])],
                "type": cls_name,
                "confidence": round(conf, 4),
            })

    # Get image size from the result
    orig_shape = result.orig_shape if results and len(results) > 0 else (0, 0)
    image_size = {"width": int(orig_shape[1]), "height": int(orig_shape[0])}

    return {"regions": regions, "image_size": image_size}


IMAGE_EXTENSIONS = {".png", ".jpg", ".jpeg", ".tiff", ".tif", ".bmp", ".webp"}


def main():
    if len(sys.argv) < 2:
        print("Usage: entrypoint.py <image_path_or_directory>", file=sys.stderr)
        sys.exit(1)

    input_path = Path(sys.argv[1])
    device = os.environ.get("DEVICE", "cpu")

    model = load_model()

    if input_path.is_dir():
        # Batch mode: process all images in directory
        images = sorted(
            p for p in input_path.iterdir()
            if p.suffix.lower() in IMAGE_EXTENSIONS
        )
        for img_path in images:
            result = detect_regions(model, img_path, device)
            result["file"] = img_path.name
            print(json.dumps(result, ensure_ascii=False))
    else:
        # Single file mode
        result = detect_regions(model, input_path, device)
        json.dump(result, sys.stdout, ensure_ascii=False)


if __name__ == "__main__":
    main()
