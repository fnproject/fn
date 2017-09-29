FROM jjanzic/docker-python3-opencv

RUN mkdir /code
ADD . /code/
WORKDIR /code
RUN pip3 install -r requirements.txt

WORKDIR /code/
ENTRYPOINT ["python3", "func.py"]
