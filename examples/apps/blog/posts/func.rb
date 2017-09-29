require 'json'
require 'sequel'

DB = Sequel.connect("mysql2://docker.for.mac.localhost/blog?user=#{ENV['DB_USER']}&password=#{ENV['DB_PASS']}")

items = DB[:posts]

rlist = []
items.each_with_index do |x,i|
	STDERR.puts "item: #{x}"
	rlist << x
end
r = {posts: rlist}
puts r.to_json
