FROM golang:latest

# Install packages
RUN curl -sL https://deb.nodesource.com/setup_10.x | bash -
RUN apt-get install -y git nodejs netcat
RUN go get -u github.com/golang/dep/cmd/dep

# Copy in source and install deps
RUN mkdir -p /go/src/buildspace
COPY ./package.json /go/src/buildspace/
WORKDIR /go/src/buildspace
RUN npm install -g serverless && npm install
COPY ./ /go/src/buildspace/
RUN dep ensure
