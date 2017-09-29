require 'uri'
require 'net/http'
require 'json'

name = "I love rubies"

payload = STDIN.read
if payload != ""
    payload = JSON.parse(payload)
    name = payload['name']
end


def open(url)
    Net::HTTP.get(URI.parse(url))
end
h = "docker.for.mac.localhost" # ENV['HOSTNAME']

header = open("http://#{h}:8080/r/#{ENV['FN_APP_NAME']}/header") # todo: grab env vars to construct this
puts header

puts "Hello, #{name}! YOOO"

footer = open("http://#{h}:8080/r/#{ENV['FN_APP_NAME']}/footer") # todo: grab env vars to construct this
puts footer
