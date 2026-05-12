#!/usr/bin/env python3
"""DocLayout-YOLO layout detection with smart post-processing.

Takes a page image and outputs JSON with detected document regions,
refined by classical CV post-processing (separator detection, column/row
splitting, zone classification, reading order).

Usage:
    python entrypoint.py /input/page.png
    python entrypoint.py --conf 0.15 --direction rtl /input/page.png
    python entrypoint.py --no-postprocess /input/page.png
    python entrypoint.py --self-test
"""

import argparse
import json
import os
import sys
from pathlib import Path

import cv2
import numpy as np
from doclayout_yolo import YOLOv10


# ---------------------------------------------------------------------------
# YOLO detection
# ---------------------------------------------------------------------------

def load_model():
    model_path = os.environ.get(
        "MODEL", "/models/doclayout_yolo_docstructbench_imgsz1024.pt"
    )
    return YOLOv10(model_path)


def detect_regions(model, image_path, device, conf, iou, imgsz, max_det):
    """Run YOLO detection, return raw regions + image_size."""
    results = model.predict(
        str(image_path), imgsz=imgsz, conf=conf, iou=iou,
        max_det=max_det, device=device, verbose=False,
    )

    regions = []
    if results and len(results) > 0:
        result = results[0]
        boxes = result.boxes
        for i in range(len(boxes)):
            xyxy = boxes.xyxy[i].tolist()
            regions.append({
                "bbox": [int(xyxy[0]), int(xyxy[1]), int(xyxy[2]), int(xyxy[3])],
                "type": result.names[int(boxes.cls[i])],
                "confidence": round(float(boxes.conf[i]), 4),
            })

    orig_shape = result.orig_shape if results and len(results) > 0 else (0, 0)
    image_size = {"width": int(orig_shape[1]), "height": int(orig_shape[0])}
    return regions, image_size


# ---------------------------------------------------------------------------
# Post-processing pipeline
# ---------------------------------------------------------------------------

def postprocess(regions, image_path, direction="rtl"):
    """Refine YOLO detections with classical CV post-processing.

    Returns (refined_regions, reading_order, separator_y, separator_method, stats).
    """
    image = cv2.imread(str(image_path))
    if image is None:
        # Can't load image — return raw regions unchanged
        for r in regions:
            r["raw_type"] = r["type"]
            r["zone"] = "body"
        regions = _assign_ids(regions)
        order = [r["id"] for r in regions]
        return regions, order, None, None, _empty_stats(len(regions))

    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    h, w = gray.shape

    regions_before = len(regions)

    # Step 1: Separator detection
    separator_y, separator_method = _detect_separator(regions, gray, w, h)

    # Step 2: Zone classification
    regions = _classify_zones(regions, separator_y, w, h)

    # Step 3: Column splitting for wide entry/footnote regions
    regions = _split_columns(regions, gray, w, zone="entry")

    # Step 4: Row splitting within columns
    regions = _split_rows(regions, gray, zone="entry")
    regions = _split_rows(regions, gray, zone="footnote")

    # Step 5: Type refinement based on zone
    regions = _refine_types(regions)

    # Step 6: Assign IDs and reading order
    regions = _assign_ids(regions)
    reading_order = _compute_reading_order(regions, direction)

    columns = _count_columns(regions)
    stats = {
        "separator_found": separator_y is not None,
        "separator_method": separator_method,
        "columns_detected": columns,
        "regions_before_split": regions_before,
        "regions_after_split": len(regions),
    }
    return regions, reading_order, separator_y, separator_method, stats


def _empty_stats(n):
    return {
        "separator_found": False, "separator_method": None,
        "columns_detected": 1, "regions_before_split": n, "regions_after_split": n,
    }


# ---------------------------------------------------------------------------
# Step 1: Separator detection
# ---------------------------------------------------------------------------

def _detect_separator(regions, gray, w, h):
    """Find the horizontal separator line that divides entries from footnotes."""
    # Method 1: From YOLO — look for thin, wide "abandon" region
    for r in regions:
        if r["type"] != "abandon":
            continue
        x1, y1, x2, y2 = r["bbox"]
        rw = x2 - x1
        rh = y2 - y1
        if rw > w * 0.6 and rh < h * 0.03:
            return (y1 + y2) // 2, "yolo"

    # Method 2: Pixel scanning fallback
    _, bw = cv2.threshold(gray, 0, 255, cv2.THRESH_BINARY_INV + cv2.THRESH_OTSU)
    margin = int(w * 0.1)
    center_strip = bw[:, margin:w - margin]
    strip_w = center_strip.shape[1]
    if strip_w <= 0:
        return None, None

    # Scan in bands of 3 pixels looking for high horizontal density
    band_h = 3
    for y in range(int(h * 0.2), int(h * 0.85), band_h):
        if y + band_h > h:
            break
        band = center_strip[y:y + band_h, :]
        density = np.count_nonzero(band) / (band_h * strip_w)
        if density > 0.4:
            # Check that the region above and below is NOT dense (it's a line, not text)
            above = center_strip[max(0, y - 15):y, :]
            below = center_strip[y + band_h:min(h, y + band_h + 15), :]
            above_density = np.count_nonzero(above) / max(above.size, 1)
            below_density = np.count_nonzero(below) / max(below.size, 1)
            if above_density < 0.15 and below_density < 0.15:
                return y + band_h // 2, "pixel"

    return None, None


# ---------------------------------------------------------------------------
# Step 2: Zone classification
# ---------------------------------------------------------------------------

def _classify_zones(regions, separator_y, w, h):
    """Assign zone to each region based on separator position."""
    for r in regions:
        r["raw_type"] = r["type"]
        x1, y1, x2, y2 = r["bbox"]
        cy = (y1 + y2) // 2

        # Page number: bottom 5%, small region
        if cy > h * 0.95 and (y2 - y1) < h * 0.05:
            r["zone"] = "page_number"
        # Header: top 10% or YOLO says title/section-header
        elif cy < h * 0.10 or r["type"] in ("title", "section-header", "page-header"):
            r["zone"] = "header"
        # Separator itself
        elif r["type"] == "abandon" and (x2 - x1) > w * 0.6 and (y2 - y1) < h * 0.03:
            r["zone"] = "separator"
        # If we have a separator, split by it
        elif separator_y is not None:
            if cy < separator_y:
                r["zone"] = "entry"
            else:
                r["zone"] = "footnote"
        else:
            r["zone"] = "body"

    return regions


# ---------------------------------------------------------------------------
# Step 3: Column splitting
# ---------------------------------------------------------------------------

def _split_columns(regions, gray, page_w, zone="entry"):
    """Split wide regions into left/right columns using vertical projection."""
    result = []
    for r in regions:
        if r["zone"] != zone:
            result.append(r)
            continue

        x1, y1, x2, y2 = r["bbox"]
        rw = x2 - x1
        if rw < page_w * 0.6:
            result.append(r)
            continue

        # Crop and analyze
        crop = gray[y1:y2, x1:x2]
        if crop.size == 0:
            result.append(r)
            continue

        _, bw = cv2.threshold(crop, 0, 255, cv2.THRESH_BINARY_INV + cv2.THRESH_OTSU)
        vproj = np.sum(bw > 0, axis=0)  # vertical projection profile

        # Look for gap in the middle 40% (between 30%-70%)
        mid_start = int(len(vproj) * 0.30)
        mid_end = int(len(vproj) * 0.70)
        mid_section = vproj[mid_start:mid_end]
        if len(mid_section) == 0:
            result.append(r)
            continue

        avg = np.mean(vproj)
        if avg == 0:
            result.append(r)
            continue

        threshold = avg * 0.10
        min_gap_w = max(int(rw * 0.02), 3)

        gap_start = None
        best_gap = None
        for i, v in enumerate(mid_section):
            if v < threshold:
                if gap_start is None:
                    gap_start = i
            else:
                if gap_start is not None:
                    gap_w = i - gap_start
                    if gap_w >= min_gap_w:
                        if best_gap is None or gap_w > best_gap[1] - best_gap[0]:
                            best_gap = (gap_start, i)
                    gap_start = None
        if gap_start is not None:
            gap_w = len(mid_section) - gap_start
            if gap_w >= min_gap_w:
                if best_gap is None or gap_w > best_gap[1] - best_gap[0]:
                    best_gap = (gap_start, len(mid_section))

        if best_gap is None:
            result.append(r)
            continue

        # Split at the gap center
        split_x = x1 + mid_start + (best_gap[0] + best_gap[1]) // 2

        left = {
            "bbox": [x1, y1, split_x, y2], "type": r["type"],
            "confidence": 0.0, "raw_type": "split", "zone": r["zone"],
        }
        right = {
            "bbox": [split_x, y1, x2, y2], "type": r["type"],
            "confidence": 0.0, "raw_type": "split", "zone": r["zone"],
        }
        result.extend([left, right])

    return result


# ---------------------------------------------------------------------------
# Step 4: Row splitting
# ---------------------------------------------------------------------------

def _split_rows(regions, gray, zone="entry"):
    """Split tall regions into individual rows using horizontal projection."""
    result = []
    for r in regions:
        if r["zone"] != zone:
            result.append(r)
            continue

        x1, y1, x2, y2 = r["bbox"]
        rh = y2 - y1
        if rh < 30:  # too small to split
            result.append(r)
            continue

        crop = gray[y1:y2, x1:x2]
        if crop.size == 0:
            result.append(r)
            continue

        _, bw = cv2.threshold(crop, 0, 255, cv2.THRESH_BINARY_INV + cv2.THRESH_OTSU)
        hproj = np.sum(bw > 0, axis=1)  # horizontal projection

        avg = np.mean(hproj)
        if avg == 0:
            result.append(r)
            continue

        threshold = avg * 0.15
        min_gap_h = max(int(rh * 0.005), 2)

        # Find all gaps
        gaps = []
        gap_start = None
        for i, v in enumerate(hproj):
            if v < threshold:
                if gap_start is None:
                    gap_start = i
            else:
                if gap_start is not None:
                    if i - gap_start >= min_gap_h:
                        gaps.append((gap_start, i))
                    gap_start = None

        if not gaps:
            result.append(r)
            continue

        # Split at each gap
        splits = [y1]
        for gs, ge in gaps:
            splits.append(y1 + (gs + ge) // 2)
        splits.append(y2)

        for i in range(len(splits) - 1):
            row_y1 = splits[i]
            row_y2 = splits[i + 1]
            if row_y2 - row_y1 < 5:  # skip tiny slivers
                continue
            result.append({
                "bbox": [x1, row_y1, x2, row_y2], "type": r["type"],
                "confidence": 0.0, "raw_type": "split", "zone": r["zone"],
            })

    return result


# ---------------------------------------------------------------------------
# Step 5: Type refinement
# ---------------------------------------------------------------------------

def _refine_types(regions):
    """Override region types based on zone assignment."""
    for r in regions:
        zone = r.get("zone", "body")
        if zone == "header":
            r["type"] = "header"
        elif zone == "entry":
            r["type"] = "entry"
        elif zone == "separator":
            r["type"] = "separator"
        elif zone == "footnote":
            r["type"] = "footnote"
        elif zone == "page_number":
            r["type"] = "page_number"
        # "body" keeps original type
    return regions


# ---------------------------------------------------------------------------
# Step 6: IDs and reading order
# ---------------------------------------------------------------------------

def _assign_ids(regions):
    for i, r in enumerate(regions):
        r["id"] = f"r{i + 1}"
    return regions


def _compute_reading_order(regions, direction="rtl"):
    """Sort regions by zone priority, then spatially within each zone."""
    zone_priority = {"header": 0, "entry": 1, "body": 2, "separator": 3,
                     "footnote": 4, "page_number": 5}

    def sort_key(r):
        zp = zone_priority.get(r.get("zone", "body"), 2)
        x1, y1, x2, y2 = r["bbox"]
        cy = (y1 + y2) // 2
        cx = (x1 + x2) // 2
        # Within entry zone: sort by row (y), then by column (x based on direction)
        if direction == "rtl":
            return (zp, cy, -cx)  # right-to-left: higher x first
        else:
            return (zp, cy, cx)   # left-to-right: lower x first

    sorted_regions = sorted(regions, key=sort_key)
    return [r["id"] for r in sorted_regions]


def _count_columns(regions):
    """Estimate number of columns from split regions."""
    entry_regions = [r for r in regions if r.get("zone") == "entry"]
    if not entry_regions:
        return 1
    xs = set()
    for r in entry_regions:
        xs.add(r["bbox"][0])  # left edge
    # If we see 2+ distinct left edges (more than 20% apart), it's multi-column
    if len(xs) >= 2:
        xs_sorted = sorted(xs)
        for i in range(1, len(xs_sorted)):
            if xs_sorted[i] - xs_sorted[0] > 50:
                return 2
    return 1


# ---------------------------------------------------------------------------
# Self-test
# ---------------------------------------------------------------------------

def self_test():
    """Create synthetic image with known structure and verify post-processing."""
    h, w = 600, 400
    img = np.ones((h, w), dtype=np.uint8) * 255  # white page

    # Header text at top
    cv2.putText(img, "HEADER", (150, 30), cv2.FONT_HERSHEY_SIMPLEX, 0.8, 0, 2)
    # Entry text in two columns
    for row in range(3):
        y = 100 + row * 60
        cv2.putText(img, f"R{row}C1", (30, y), cv2.FONT_HERSHEY_SIMPLEX, 0.6, 0, 1)
        cv2.putText(img, f"R{row}C2", (230, y), cv2.FONT_HERSHEY_SIMPLEX, 0.6, 0, 1)
    # Separator line
    cv2.line(img, (20, 350), (380, 350), 0, 2)
    # Footnote text
    cv2.putText(img, "Footnote 1", (30, 400), cv2.FONT_HERSHEY_SIMPLEX, 0.5, 0, 1)

    # Save temp image
    tmp = "/tmp/_doclayout_selftest.png"
    cv2.imwrite(tmp, img)

    # Simulate YOLO detections (since we can't run the model in self-test)
    regions = [
        {"bbox": [100, 5, 300, 45], "type": "title", "confidence": 0.9},
        {"bbox": [10, 70, 390, 310], "type": "plain text", "confidence": 0.8},
        {"bbox": [10, 340, 390, 360], "type": "abandon", "confidence": 0.7},
        {"bbox": [10, 370, 390, 430], "type": "footnote", "confidence": 0.75},
    ]

    refined, order, sep_y, sep_method, stats = postprocess(regions, tmp, "ltr")
    os.remove(tmp)

    errors = []
    if sep_y is None:
        errors.append("separator not detected")
    if not stats["separator_found"]:
        errors.append("stats.separator_found is False")

    zones = {r["zone"] for r in refined}
    if "header" not in zones:
        errors.append("no header zone found")
    if "separator" not in zones:
        errors.append("no separator zone found")
    if "footnote" not in zones:
        errors.append("no footnote zone found")

    if len(order) != len(refined):
        errors.append(f"reading_order length {len(order)} != regions {len(refined)}")

    if errors:
        print("SELF-TEST FAILED:", "; ".join(errors), file=sys.stderr)
        sys.exit(1)

    print(json.dumps({
        "status": "ok",
        "regions": len(refined),
        "zones": sorted(zones),
        "separator_y": sep_y,
        "separator_method": sep_method,
        "stats": stats,
    }))
    sys.exit(0)


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

IMAGE_EXTENSIONS = {".png", ".jpg", ".jpeg", ".tiff", ".tif", ".bmp", ".webp"}


def main():
    parser = argparse.ArgumentParser(description="DocLayout-YOLO layout detection")
    parser.add_argument("input", nargs="?", help="Image path or directory")
    parser.add_argument("--conf", type=float, default=0.2,
                        help="Confidence threshold (default: 0.2)")
    parser.add_argument("--iou", type=float, default=0.7,
                        help="IoU threshold for NMS (default: 0.7)")
    parser.add_argument("--imgsz", type=int, default=1024,
                        help="Input image size (default: 1024)")
    parser.add_argument("--max-det", type=int, default=300,
                        help="Maximum detections per image (default: 300)")
    parser.add_argument("--direction", choices=["rtl", "ltr"], default="rtl",
                        help="Text direction for reading order (default: rtl)")
    parser.add_argument("--no-postprocess", action="store_true",
                        help="Skip post-processing, output raw YOLO detections")
    parser.add_argument("--self-test", action="store_true",
                        help="Run self-test with synthetic image")
    args = parser.parse_args()

    if args.self_test:
        self_test()
        return

    if not args.input:
        parser.error("input is required (unless --self-test)")

    input_path = Path(args.input)
    device = os.environ.get("DEVICE", "cpu")
    model = load_model()

    if input_path.is_dir():
        images = sorted(
            p for p in input_path.iterdir()
            if p.suffix.lower() in IMAGE_EXTENSIONS
        )
        for img_path in images:
            result = _process_one(model, img_path, device, args)
            result["file"] = img_path.name
            print(json.dumps(result, ensure_ascii=False))
    else:
        result = _process_one(model, input_path, device, args)
        json.dump(result, sys.stdout, ensure_ascii=False)


def _process_one(model, image_path, device, args):
    """Process a single image: YOLO detection + optional post-processing."""
    regions, image_size = detect_regions(
        model, image_path, device, args.conf, args.iou, args.imgsz, args.max_det
    )

    if args.no_postprocess:
        return {"regions": regions, "image_size": image_size}

    refined, reading_order, sep_y, sep_method, stats = postprocess(
        regions, image_path, args.direction
    )

    result = {
        "regions": refined,
        "reading_order": reading_order,
        "image_size": image_size,
        "post_processing": stats,
    }
    if sep_y is not None:
        result["separator_y"] = sep_y
    return result


if __name__ == "__main__":
    main()
