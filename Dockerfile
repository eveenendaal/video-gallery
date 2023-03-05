FROM node:lts-slim

COPY . .

CMD ["npm", "start"]
