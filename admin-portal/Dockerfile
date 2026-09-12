# admin-portal/Dockerfile
FROM node:20-alpine

WORKDIR /app

# Copy configuration files first
COPY package.json ./

# Install dependencies inside the Linux container environment
RUN npm install

# Copy the rest of the application files
COPY . .

EXPOSE 5174

CMD ["npm", "run", "dev"]