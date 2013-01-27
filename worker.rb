require 'rest'
require 'sinatra'

# The backend would do this part before execute the worker
########################################################
rest = Rest::Client.new
rest.logger.level = Logger::DEBUG

public_dns = rest.get("http://169.254.169.254/latest/meta-data/public-hostname").body

puts "public dns name: #{public_dns}"
port = rand(50000..55000)
puts "port: #{port}"

project_id = params[:project_id] || "4fd2729368a0197d1102056b" # // this is my Default project
token = params[:token] || "MWx0VfngzsCu0W8NAYw7S2lNrgo"
code_name = params[:code_name] || "sinatra"

query = "?project_id=#{project_id}&token=#{token}&code_name=#{code_name}"
response = rest.post(
#   "http://localhost:80/#{query}",
    "http://router.irondns.info/#{query}",
	headers: {"Iron-Router"=>"YES!", "Content-Type"=>"application/json"},
	body: {"host"=>"routertest.irondns.info", "dest"=>"#{public_dns}:#{port}"})
puts "body:"
puts response.body
puts "\n\n=======\n\n"

STDOUT.flush

# Now we start the actual worker
##################################################################3
  
ENV['PORT'] = port.to_s # for sinatra
my_app = Sinatra.new do
    set :port, port
  get('/') { "hi" }
  get('/*') { "you passed in #{params[:splat].inspect}" }
end
my_app.run!

