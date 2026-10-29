FROM debian:latest
LABEL maintainer="Bokanev Artem Vladimirovich"
ENV MASTER="false"
ENV SLAVE="false"
ENV DEBUG="false"
ENV MASTERENDPOINT="localhost:8080"
ENV MASTERPORT="8080"
RUN apt update && apt upgrade -y && apt install bash
RUN groupadd -r minioclient && useradd -r -g minioclient minioclient
COPY ./ /home/minioclient
WORKDIR /home/minioclient
RUN chown -R minioclient:minioclient /home/minioclient/
USER minioclient
RUN chmod +x entrypoint.sh 
RUN find ./ -name minioclient -exec chmod +x {} \;
CMD [ "./entrypoint.sh" ]
