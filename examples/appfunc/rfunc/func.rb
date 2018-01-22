require 'sinatra'
set :port, ENV['PORT'] || 8080
set :bind, '0.0.0.0' # required

get '/hello' do
	STDERR.puts "in /hello, port=" + settings.port
  'Put this in your pipe & smoke it!'
end

get '/' do 
	'yo!'
end