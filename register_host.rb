require 'rest'
require 'uber_config'

@config = UberConfig.load('config_development.yml')

rest = Rest::Client.new
rest.logger.level = Logger::DEBUG

project_id = @config[:project_id]
token = @config[:token]
# host name
host_name = @config[:host_name] || "routertest.irondns.info"
# which worker to run
code_name = @config[:code_name] || "sinatra"


response = rest.post(
# "http://localhost:8080/",
    "http://router.irondns.info/?project_id=#{project_id}&token=#{token}&code_name=#{code_name}",
	headers: {"Iron-Router"=>"register"},
	body: {"host"=>host_name, "code"=>code_name)
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

