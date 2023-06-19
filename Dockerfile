FROM node:19-slim

COPY . .

CMD ["npm", "start"]
