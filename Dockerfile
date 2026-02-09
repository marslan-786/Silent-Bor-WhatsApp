# ═══════════════════════════════════════════════════════════
# 1. Stage: Go Builder
# ═══════════════════════════════════════════════════════════
FROM golang:1.24-bookworm AS go-builder

# SQLite کے لیے GCC اور CGO ضروری ہیں
RUN apt-get update && apt-get install -y \
    gcc libc6-dev git libsqlite3-dev ffmpeg \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# تمام Go فائلز کاپی کریں (main, commands, groups, owner, types, lid_system)
COPY . .

# گو موڈیولز کو انیشلائز کریں (تمام لائبریریاں رکھی گئی ہیں جیسا آپ نے کہا)
RUN rm -f go.mod go.sum || true
RUN go mod init impossible-bot && \
    go get go.mau.fi/whatsmeow@latest && \
    go get github.com/mattn/go-sqlite3@latest && \
    go get github.com/gorilla/websocket@latest && \
    go get google.golang.org/protobuf/proto@latest && \
    go get go.mongodb.org/mongo-driver/mongo@latest && \
    go get go.mongodb.org/mongo-driver/bson@latest && \
    go get github.com/redis/go-redis/v9@latest && \
    go get github.com/gin-gonic/gin@latest && \
    go get github.com/lib/pq@latest && \
    go get github.com/showwin/speedtest-go && \
    go get google.golang.org/genai && \
    go mod tidy

# Binary Build کریں
RUN CGO_ENABLED=1 GOOS=linux go build -v -ldflags="-s -w" -o bot .

# (Node Builder اسٹیج ہٹا دیا گیا ہے کیونکہ اب LID سسٹم Go میں ہے)

# ═══════════════════════════════════════════════════════════
# 2. Stage: Final Runtime (Python + System Tools)
# ═══════════════════════════════════════════════════════════
FROM python:3.10-slim-bookworm

ENV PYTHONUNBUFFERED=1

# 🛠️ سسٹم لائبریریز (Node.js رکھا ہے کیونکہ yt-dlp کو ضرورت پڑ سکتی ہے)
RUN apt-get update && apt-get install -y \
    ffmpeg imagemagick curl sqlite3 libsqlite3-0 \
    nodejs npm \
    atomicparsley \
    ca-certificates libgomp1 megatools libwebp-dev webp \
    libwebpmux3 libwebpdemux2 libsndfile1 \
    && rm -rf /var/lib/apt/lists/*

# 🛠️ CRITICAL FIX: yt-dlp needs 'node' alias
RUN ln -sf /usr/bin/nodejs /usr/local/bin/node

# yt-dlp انسٹالیشن (Latest)
RUN curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp \
    && chmod a+rx /usr/local/bin/yt-dlp

# 🐍 Python Libraries (رکھی گئی ہیں تاکہ بعد میں AI فیچرز ایڈ کیے جا سکیں)
RUN pip3 install --no-cache-dir \
    torch torchaudio --index-url https://download.pytorch.org/whl/cpu \
    && pip3 install --no-cache-dir \
    fastapi uvicorn python-multipart requests \
    faster-whisper scipy gTTS playwright

# Playwright Browsers
RUN playwright install --with-deps chromium

WORKDIR /app

# ✅ صرف وہ فائلز کاپی کریں جو اب ہمارے پاس موجود ہیں
# 1. Go Binary
COPY --from=go-builder /app/bot ./bot

# 2. Assets (Root Directory میں)
COPY index.html ./index.html
COPY pic.png ./pic.png

# 3. Python Scripts (اگر آپ نے فی الحال نہیں بنائے تو یہ لائنز کمنٹ کر دیں ورنہ ایرر آئے گا)
# اگر یہ فائلز موجود ہیں تو ہی ان کمنٹس کو ہٹائیں:
# COPY ai_engine.py ./ai_engine.py
# COPY tiktok_nav.py ./tiktok_nav.py
# COPY browser_dl.py ./browser_dl.py

# 4. Data Volume Directory (SQLite کے لیے)
RUN mkdir -p /data

# Permissions set کریں
RUN chmod +x /app/bot

ENV PORT=8080
EXPOSE 8080

# بوٹ چلائیں
CMD ["/app/bot"]
