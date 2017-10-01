FROM python:3.6.2

RUN mkdir /code
ADD . /code/
WORKDIR /code
RUN pip3 install -r requirements.txt

ENTRYPOINT ["python3", "func.py"]
