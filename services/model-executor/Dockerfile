FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

ENV GRPC_PORT=50051 \
    HTTP_PORT=8085 \
    MAX_WORKERS=10 \
    EXECUTOR_ID=executor-0

EXPOSE 50051 8085

CMD ["python", "-m", "server.main"]
