# frontend/Dockerfile
FROM node:20-alpine

WORKDIR /app

# Copy package configuration first
COPY package.json ./

# Install dependencies inside the Linux container environment
RUN npm install

# Copy the rest of the source code
COPY . .

EXPOSE 5173

CMD ["npm", "run", "dev"]