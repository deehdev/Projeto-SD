FROM python:3.12-slim

WORKDIR /app

RUN pip install pyzmq

COPY . .

CMD ["python", "-u", "bot.py"]