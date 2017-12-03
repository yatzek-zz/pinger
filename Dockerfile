FROM sickp/alpine-sshd:7.5
RUN passwd -d root
COPY keys/docker_id_rsa.pub /root/.ssh/authorized_keys