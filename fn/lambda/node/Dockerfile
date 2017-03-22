FROM node:4-alpine

WORKDIR /function

# Install ImageMagick and AWS SDK as provided by Lambda.
RUN apk update && apk --no-cache add imagemagick
RUN npm install aws-sdk@2.2.32 imagemagick && npm cache clear

# ironcli should forbid this name
ADD bootstrap.js /function/lambda-bootstrap.js

# Run the handler, with a payload in the future.
ENTRYPOINT ["node", "./lambda-bootstrap"]
