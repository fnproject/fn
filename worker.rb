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

response = rest.post(
# "http://localhost:8080/",
    "http://router.irondns.info/?project_id=#{params[:project_id]}&token=#{params[:token]}",
	headers: {"Iron-Router"=>"YES!"},
	body: {"host"=>"routertest.irondns.info", "dest"=>"#{public_dns}:#{port}"})
puts "body:"
puts response.body

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

