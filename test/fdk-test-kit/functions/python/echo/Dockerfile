FROM python:3.6.2


RUN mkdir /code
ADD . /code/
WORKDIR /code/
RUN pip install -r requirements.txt
WORKDIR /code/

ENTRYPOINT ["python3", "func.py"]
