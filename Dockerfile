FROM node:18-slim

COPY . .

CMD ["npm", "start"]
