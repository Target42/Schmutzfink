FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates libgomp1 curl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --uid 1000 --create-home schmutzfink

WORKDIR /app
COPY schmutzfink /app/schmutzfink
COPY web /app/web
COPY vendor /app/vendor
COPY .env /app/.env

RUN chmod +x /app/schmutzfink \
    && mkdir -p /app/data/objects \
    && chown -R schmutzfink:schmutzfink /app

USER schmutzfink
EXPOSE 8787
VOLUME /app/data

ENV HTTP_ADDR=:8787 \
    STORAGE_BACKEND=local \
    STORAGE_DIR=/app/data/objects \
    WEB_DIST=/app/web/dist \
    ONNXRUNTIME_OPENCLIP_CACHE_DIR=/app/vendor/openclip \
    ONNXRUNTIME_LIB_PATH=/app/vendor/onnxruntime/libonnxruntime.so \
    TOKENIZERS_LIB_PATH=/app/vendor/tokenizers/libtokenizers.so \
    LD_LIBRARY_PATH=/app/vendor/onnxruntime

HEALTHCHECK --interval=30s --timeout=5s --start-period=40s --retries=3 \
    CMD curl -fsS http://127.0.0.1:8787/api/health || exit 1

CMD ["/app/schmutzfink"]
