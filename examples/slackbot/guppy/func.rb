require_relative 'bundle/bundler/setup'
require 'slack_webhooks'

payload = STDIN.read
#STDERR.puts "PAYLOAD: #{payload}"

images = JSON.load(File.open('commands.json'))

response = {}
attachment = {
    "fallback" => "wat?!",
    "text" => "",
    "image_url" => "http://i.imgur.com/7kZ562z.jpg"
}

help = "Available options are:\n"
images.each_key { |k| help << "* #{k}\n" }

response = {}
a = []
response[:attachments] = a

if payload.nil? || payload.strip == ""
  response[:text] = help
  a << attachment
  puts response.to_json
  exit
end

sh = SlackWebhooks::Hook.new('guppy', payload, "")
r = images[sh.text]
if r
  a << {image_url: r['image_url'], text: ""} 
  response[:response_type] = "in_channel"
  response[:text] = "guppy #{sh.text}" 
else
  response[:text] = help
  a << attachment
end

puts response.to_json



