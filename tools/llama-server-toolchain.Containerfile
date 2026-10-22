FROM ubuntu:22.04@sha256:0a9887931b0e38af2e3487ed39e29436b0535c4f3f52e57745baaf79d1898956
RUN apt-get update -qq \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq build-essential cmake git software-properties-common \
  && add-apt-repository -y ppa:ubuntu-toolchain-r/test \
  && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq gcc-14 g++-14 \
  && rm -rf /var/lib/apt/lists/*
ENV CC=gcc-14 CXX=g++-14
