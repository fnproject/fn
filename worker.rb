require 'rest'
require 'sinatra'

# Now we start the actual worker
##################################################################3
  
ENV['PORT'] = port.to_s # for sinatra
my_app = Sinatra.new do
    set :port, port
  get('/') { "hi" }
  get('/*') { "you passed in #{params[:splat].inspect}" }
end
my_app.run!

