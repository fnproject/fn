FROM iron/ruby:dev

WORKDIR /worker
ADD Gemfile* /worker/
RUN bundle install

ADD . /worker/

ENTRYPOINT ["ruby", "hello.rb"]
