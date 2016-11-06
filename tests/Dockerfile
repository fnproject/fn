FROM iron/ruby:dev

WORKDIR /app
ADD Gemfile* /app/
RUN bundle install

ADD . /app/

# ENTRYPOINT ["ruby", "test.rb"]
