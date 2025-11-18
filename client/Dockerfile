FROM node:18-alpine

WORKDIR /app

# Instalar dependências do ZeroMQ
RUN apk add --no-cache zeromq zeromq-dev python3 make g++ bash

# Copiar package.json e instalar dependências
COPY package*.json ./
RUN npm install --production

# Copiar todo o diretório client
COPY . .

# Comando padrão
CMD ["node", "client.js"]
