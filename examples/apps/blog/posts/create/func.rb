require 'json'
require 'sequel'

DB = Sequel.connect("mysql2://docker.for.mac.localhost/blog?user=#{ENV['DB_USER']}&password=#{ENV['DB_PASS']}")

payload = STDIN.read
if payload == ""
    puts ({"error" => "Invalid input"}).to_json
    exit 1
end

payload = JSON.parse(payload)

# create a dataset from the items table
items = DB[:posts]

# populate the table
items.insert(:title => payload['title'], :body => payload['body'])

puts ({"status"=>"success", "message" => "Post inserted successfully."}).to_json
