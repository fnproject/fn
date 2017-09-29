require "sequel"

DB = Sequel.connect("mysql2://docker.for.mac.localhost/blog?user=#{ENV['DB_USER']}&password=#{ENV['DB_PASS']}")

# create a posts table
DB.create_table :posts do
  primary_key :id
  String :title
  String :body
end
