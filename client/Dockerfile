FROM node:18-slim

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm install --production

COPY . .

CMD ["node", "client.js"]