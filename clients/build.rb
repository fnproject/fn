require 'yaml'
require 'open-uri'
require 'http'
require 'fileutils'
require 'openssl'

require_relative 'utils.rb'

swaggerUrl = "https://raw.githubusercontent.com/iron-io/functions/master/docs/swagger.yml"
spec = YAML.load(open(swaggerUrl))
version = spec['info']['version']
puts "VERSION: #{version}"

# Can pass in a particular language to only do that one
only = ARGV[0]
puts "only building: #{only}" if only

# Keep getting cert errors??  Had to do this to work around it:
ctx = OpenSSL::SSL::SSLContext.new
ctx.verify_mode = OpenSSL::SSL::VERIFY_NONE

def clone(lang)
  Dir.chdir 'tmp'
  ldir = "functions_#{lang}"
  if !Dir.exist? ldir
    cmd = "git clone https://github.com/iron-io/#{ldir}"
    stream_exec(cmd)
  else
    Dir.chdir ldir
    cmd = "git pull"
    stream_exec(cmd)
    Dir.chdir '../'
  end
  Dir.chdir '../'
end

FileUtils.mkdir_p 'tmp'
languages = nil
if only
  languages = [only]
else 
  languages = JSON.parse(HTTP.get("https://generator.swagger.io/api/gen/clients", ssl_context: ctx).body)
end
languages.each do |l|
  puts l
  lshort = l
  # lang_options = JSON.parse(HTTP.get("https://generator.swagger.io/api/gen/clients/#{l}", ssl_context: ctx).body)
  # p lang_options
  # only going to do ruby and go for now
  glob_pattern = ["**", "*"]
  copy_dir = "."
  options = {}
  skip_files = []
  deploy = []
  case l
  when 'go'
    clone(lshort)
    glob_pattern = ['functions', "**", "*.go"]
    copy_dir = "."
    options['packageName'] = 'functions'
    options['packageVersion'] = version
  when 'ruby'
    clone(l)
    fruby = "functions_ruby"
    gem_name = "iron_functions"
    glob_pattern = ["**", "*.rb"] # just rb files
    skip_files = ["#{gem_name}.gemspec"]
    deploy = ["gem build #{gem_name}.gemspec", "gem push #{gem_name}-#{version}.gem"]
    options['gemName'] = gem_name
    options['moduleName'] = "IronFunctions"
    options['gemVersion'] = version
    options['gemHomepage'] = "https://github.com/iron-io/#{fruby}"
    options['gemSummary'] = 'Ruby gem for IronFunctions'
    options['gemDescription'] = 'Ruby gem for IronFunctions.'
    options['gemAuthorEmail'] = 'travis@iron.io'
  when 'javascript'
    lshort = 'js'
    # copy_dir = "javascript-client/."
    clone(lshort)
    options['projectName'] = "iron_functions"
    deploy << "npm publish"
   else
    puts "Skipping #{l}"
    next
  end
  p options
  if l == 'go'
    puts "SKIPPING GO, it's manual for now."
    # This is using https://goswagger.io/ instead
    # TODO: run this build command instead: this works if run manually
    # dep ensure --update && docker run --rm -it  -v $HOME/dev/go:/go -w /go/src/github.com/iron-io/functions_go quay.io/goswagger/swagger generate client -f https://raw.githubusercontent.com/iron-io/functions/master/docs/swagger.yml -A functions
  else
    gen = JSON.parse(HTTP.post("https://generator.swagger.io/api/gen/clients/#{l}",
    json: {
      swaggerUrl: swaggerUrl,
      options: options,
    },
    ssl_context: ctx).body)
    p gen

    lv = "#{lshort}-#{version}"
    zipfile = "tmp/#{lv}.zip"
    stream_exec "curl -o #{zipfile} #{gen['link']} -k"
    stream_exec "unzip -o #{zipfile} -d tmp/#{lv}"
  end

  # delete the skip_files
  skip_files.each do |sf|
    begin
      File.delete("tmp/#{lv}/#{lshort}-client/" + sf)
    rescue => ex
      puts "Error deleting file: #{ex.backtrace}"
    end
  end

  # Copy into clone repos
  fj = File.join(['tmp', lv, "#{l}-client"] + glob_pattern)
  # FileUtils.mkdir_p "tmp/#{l}-copy"
  # FileUtils.cp_r(Dir.glob(fj), "tmp/#{l}-copy")
  destdir = "tmp/functions_#{lshort}"
  puts "Trying cp", "tmp/#{lv}/#{l}-client/#{copy_dir}", destdir
  FileUtils.cp_r("tmp/#{lv}/#{l}-client/#{copy_dir}", destdir)
  # Write a version file, this ensures there's always a change.
  File.open("#{destdir}/VERSION", 'w') { |file| file.write(version) }

  # Commit and push
  begin
    Dir.chdir("tmp/functions_#{lshort}")
    stream_exec "git add ."
    stream_exec "git commit -am \"Updated to api version #{version}\""
    begin
      stream_exec "git tag -a #{version} -m \"Version #{version}\""
    rescue => ex 
      puts "WARNING: Tag #{version} already exists."
    end
    stream_exec "git push --follow-tags"
    deploy.each do |d|
      stream_exec d
    end
  rescue ExecError => ex
    puts "Error: #{ex}"
    if ex.last_line.include?("nothing to commit") || ex.last_line.include?("already exists") || ex.last_line.include?("no changes added to commit")
       # ignore this
       puts "Ignoring error"
    else
       raise ex
    end
  end
  Dir.chdir("../../")

end

# Uncomment the following lines if we start using the Go lib
# Dir.chdir("../")
# stream_exec "glide up"
Dir.chdir("../tests/")
stream_exec "bundle update"
