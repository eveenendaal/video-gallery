FROM node:14-slim

COPY . .

CMD ["npm", "start"]