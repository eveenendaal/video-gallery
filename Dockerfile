FROM node:20-slim

COPY . .

CMD ["npm", "start"]
