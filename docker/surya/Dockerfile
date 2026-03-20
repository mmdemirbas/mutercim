FROM python:3.11-slim

RUN pip install --no-cache-dir surya-ocr

COPY entrypoint.py /app/entrypoint.py

ENTRYPOINT ["python", "/app/entrypoint.py"]
