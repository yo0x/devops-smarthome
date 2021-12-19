# temp stage
FROM python:3.8
RUN apt-get update && apt-get install gettext nano vim -y
ENV APP /code
RUN mkdir $APP
WORKDIR $APP
EXPOSE 5555
COPY requirements.txt .
RUN pip install -r requirements.txt
COPY . .
CMD ["uwsgi", "--ini", "app.ini", "--enable-threads"]

