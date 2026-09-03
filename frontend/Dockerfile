FROM node:20-alpine

WORKDIR /app

# Copy configuration files
COPY package.json ./

# Install dependencies inside container
RUN npm install

# Copy the rest of the application
COPY . .

# Expose Vite development server port
EXPOSE 5173

CMD ["npm", "run", "dev"]
