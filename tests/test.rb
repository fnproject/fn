# Copyright 2016 Iron.io
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

require 'test/unit'
require 'worker_ruby'
require 'net/http'

class TestTitan < Test::Unit::TestCase

    class << self
        def startup
            puts 'runs only once at start'
            
        end
        def shutdown
            puts 'runs only once at end'
        end
        
    end


  def setup
    # puts "TEST SETUP"
    # puts ENV['HOST']
    host = ENV['HOST'] || "worker-api:8080"
    IronWorker.configure do |config|
      config.host = "#{host}"
      config.scheme = "http"
      # config.debugging = true
    end

    @titan = IronWorker::TasksApi.new
    @titan_groups = IronWorker::GroupsApi.new
  end

  def wait_for_completion(j)
    max = 120
    i = 0
    while true do
      i += 1
      if i >= max 
        raise "Task never seemed to finish!"
      end
      task = @titan.groups_name_tasks_id_get(j.group_name, j.id).task
      # puts "#{task.id} task status: #{task.status}"
      if task.status != "delayed" && task.status != "queued" && task.status != "running"
        return task
      end
      sleep 1
    end
  end

  def wait_for_running(j)
    max = 120
    i = 0
    while true do
      i += 1
      if i >= max 
        raise "Task never seemed to start!"
      end
      task = @titan.groups_name_tasks_id_get(j.group_name, j.id).task
      # puts "#{task.id} task status: #{task.status}"
      if task.status == "delayed" || task.status == "queued"
        sleep 1
      elsif task.status == "running"
        return task
      else
        raise "Task already finished"
      end
    end
  end

  def post_simple_task(group_name)
    return @titan.groups_name_tasks_post(group_name, tasks: [{image: 'iron/hello', payload: {name: "Johnny Utah"}.to_json}])
  end

  def test_basics
    puts 'test_basics'
    group_name = 'basics'
    r = post_simple_task(group_name)
    assert_equal 1, r.tasks.length
    task = r.tasks[0]
    # puts "Task after put:"
    # p task
    assert task.id.length > 0
    r = @titan.groups_name_tasks_id_get(group_name, task.id)
    task = r.task
    # puts "Task after get:"
    # p task
    assert task.id.length > 0
    assert ["queued","running"].include? task.status
    puts "task created at: #{task.created_at.to_time} vs #{Time.now-5}"
    # The clock is getting off here. Wth?
    assert task.created_at.to_time > (Time.now-5)
    
    task = wait_for_completion(task)
    assert_equal "success", task.status
    assert_equal nil, task.reason
    assert task.started_at >= task.created_at
    assert task.completed_at >= task.started_at

    task = post_simple_task(group_name).tasks[0]
    task = wait_for_completion(task)
    assert_equal "success", task.status

  end

  def test_private_images
    group_name = 'private_images'
    if ENV['TEST_AUTH'] != nil
      assert_not_nil ENV['TEST_AUTH']
      assert_not_nil ENV['TEST_PRIVATE_IMAGE']
      r = @titan.groups_name_tasks_post(group_name, tasks: [{image: ENV['TEST_PRIVATE_IMAGE'], payload: {name: "Johnny Utah"}.to_json}])
      assert_equal 1, r.tasks.length
      task = r.tasks[0]
      puts "Task after put:"
      # p task
      assert task.id.length > 0
      r = @titan.groups_name_tasks_id_get(group_name, task.id)
      task = r.task
      puts "Task after get:"
      # p task
      assert task.id.length > 0
      assert ["queued","running"].include? task.status
      puts "task created at: #{task.created_at.to_time} vs #{Time.now-5}"
      # The clock is getting off here. Wth?
      assert task.created_at.to_time > (Time.now-5)

      task = wait_for_completion(task)
      # p task
      assert_equal "success", task.status
      assert task.started_at > task.created_at
      assert task.completed_at > task.started_at
    end
  end

  def test_logs
    group_name = 'logs'
    input_string = "Test Input"
    r_payload = @titan.groups_name_tasks_post(group_name, tasks: [{image: 'iron/echo:latest', payload: {input: input_string}.to_json }])
    p r_payload
    task = wait_for_completion(r_payload.tasks[0])
    # p task
    log = @titan.groups_name_tasks_id_log_get(group_name, r_payload.tasks[0].id)
    # echo image log should be exactly the input string
    assert_equal input_string, log.chomp
  end

  def test_get_tasks_by_group
    now = Time.now()
    group_name = "fortasklist"
    image = 'iron/hello'
    posted = []
    10.times do |i|
      posted << post_simple_task(group_name).tasks[0]
    end
      jarray = @titan.groups_name_tasks_get(group_name, created_after: now.to_datetime.rfc3339)
    assert_equal 10, jarray.tasks.length
    jarray.tasks.each do |task|
        p task
        assert_equal group_name, task.group_name
        assert_equal image, task.image
        assert task.created_at.to_time > now
    end
    wait_for_completion(posted.last)
  end

  def test_priorities
    # Wait a while for earlier tasks from other tests to fizzle out so that
    # their priorities do not affect things.
    # Would be nice to have some way to clean the queues without having to
    # sleep.
    sleep 5
    group_name = 'priorities'
    # Test priorities from 0-2... but how to ensure this is reproducible?
    # Post a bunch at once, then ensure start date on higher priority wins
    tasks = []
    10.times do |i|
      priority = i < 5 ? 0 : i < 8 ? 1 : 2
      j = {image: 'iron/echo:latest', priority: priority, payload: {input: "task-#{i}"}.to_json}
      p j
      tasks << j
    end
    tasks = @titan.groups_name_tasks_post(group_name, tasks: tasks).tasks
    sleep 2
    j4 = wait_for_completion(tasks[4]) # should in theory be the last one
    puts "j4: #{j4.inspect}"
    puts "j4 started_at: #{j4.started_at}"
    j9 = wait_for_completion(tasks[9])
    puts "j9 started_at: #{j9.started_at}"
    assert j9.started_at <= j4.started_at
    # Should add more comparisons here
  end

  # todo: Enable after fixing #176[]
  #def test_delay
  #  group_name = 'delay'
  #  delayed_tasks = []
  #  5.times do |i|
  #    delayed_tasks << @titan.groups_name_tasks_post(group_name, tasks: [{image: 'treeder/echo:latest', delay: 5, payload: {input: 'Test Input'}.to_json }]).tasks[0]
  #  end
  #  p "Finished queueing tasks at #{Time.now}"
  #  delayed_tasks.each do |task|
  #    task_delay = @titan.groups_name_tasks_id_get(group_name, task.id).task
  #    assert_equal "delayed", task_delay.status
  #  end

  #  sleep 2
  #  delayed_tasks.each do |task|
  #    task_delay = @titan.groups_name_tasks_id_get(group_name, task.id).task
  #    assert_equal "delayed", task_delay.status
  #  end

  #  sleep 8
  #  p "Starting to check queued/running/success status at #{Time.now}"
  #  delayed_tasks.each do |task|
  #    task_delay = @titan.groups_name_tasks_id_get(group_name, task.id).task
  #    p "status is", task_delay.status
  #    assert ["queued", "running", "success"].include?(task_delay.status)
  #    if task_delay.status == "success"
  #      assert task_delay.created_at.to_time < task_delay.started_at.to_time - 5
  #    end
  #  end
  #end
  
  def test_error
    group_name = 'error'
    r = @titan.groups_name_tasks_post(group_name, tasks: [{image: 'iron/error'}])
    p r
    task = wait_for_completion(r.tasks[0])
    assert_equal "error", task.status
    assert_equal "bad_exit", task.reason
  end

  def test_timeout
    group_name = 'timeout'
    # this one should be fine
    r1 = @titan.groups_name_tasks_post(group_name, tasks: [{
      image: 'iron/sleeper:latest',
      payload: {sleep: 10}.to_json,
      timeout: 30
      }])
    # this one should timeout
    r2 = @titan.groups_name_tasks_post(group_name, tasks: [{
      image: 'iron/sleeper:latest',
      payload: {sleep: 10}.to_json,
      timeout: 5
      }])
    task1 = wait_for_completion(r1.tasks[0])
    assert_equal "success", task1.status
    task2 = wait_for_completion(r2.tasks[0])
    assert_equal "error", task2.status
    # Fix swagger reason enum thing!
    assert_equal "timeout", task2.reason
  end
  
  # need to set retry_id field before this test can run.
  # todo: 
  def test_autoretries
    group_name = 'retries'
    r = @titan.groups_name_tasks_post(group_name, tasks: [{
        image: 'treeder/error:0.0.2', 
        retries_delay: 5,
        max_retries: 2
    }])
    task = wait_for_completion(r.tasks[0])
    assert_equal "error", task.status
    p task
    # Fix this once retries set things correctly.
    assert task.retry_at && task.retry_at != ""
    assert_equal 2, task.max_retries
    # should retry in 5 seconds. 
    task2 = @titan.groups_name_tasks_id_get(group_name, task.retry_at).task
    assert "delayed", task2.status # shouldn't start for 5 seconds
    assert task2.id == task.retry_at
    assert task2.retry_of == task.id
    task2 = wait_for_completion(task2)
    assert task.completed_at.to_time < task2.started_at.to_time - 5
    assert_equal 1, task2.max_retries # decremented
    # last retry
    task3 = @titan.groups_name_tasks_id_get(group_name, task2.retry_at).task
    assert "delayed", task3.status # shouldn't start for 5 seconds
    task3 = wait_for_completion(task3)
    assert task2.completed_at.to_time < task3.started_at.to_time - 5    
    assert task3.retry_at == nil || task3.retry_at == 0    
    assert task3.max_retries == nil || task3.max_retries == 0
  end
  
  # todo: why is this commented out?
  # todo: 
  #def test_retries
  #  group_name = 'retries'

  #  # non-existent task
  #  exc = assert_raise IronWorker::ApiError do
  #    @titan.groups_name_tasks_id_retry_post(group_name, '-1')
  #  end
  #  assert_equal 404, exc.code

  #  r = @titan.groups_name_tasks_post(group_name, tasks: [{
  #        image: 'treeder/error:0.0.2',
  #        retries_delay: 5,
  #    }])

  #  # task should not be retry-able until it is finished.
  #  exc = assert_raise IronWorker::ApiError do
  #    @titan.groups_name_tasks_id_retry_post(group_name, r.tasks[0].id)
  #  end
  #  assert_equal 409, exc.code

  #  task = wait_for_completion(r.tasks[0])
  #  assert_equal "error", task.status
  #  assert_equal 0, task.max_retries
  #  task2 = @titan.groups_name_tasks_id_retry_post(group_name, task.id).task
  #  # should retry in 5 seconds.
  #  task = @titan.groups_name_tasks_id_get(group_name, task.id).task
  #  assert task.id, task2.retry_of
  #  task2 = wait_for_completion(task2)
  #  assert_equal task.max_retries, task2.max_retries
  #end

  def test_auto_groups
    group_name = 'test_groups'
    # todo: need to delete image before this. 
    r = post_simple_task(group_name)
    r = post_simple_task(group_name)
    task = r.tasks[0]
    iw = @titan_groups.groups_name_get(group_name)
    assert_not_nil iw.group
    assert iw.group.name == group_name
    r = @titan_groups.groups_name_get(group_name)
    assert_equal group_name, r.group.name
    wait_for_completion(task)
  end

  # Try cancelling completed task
  def test_cancel_succeeded
    group_name = 'cancellation'

    r = post_simple_task(group_name)
    wait_for_completion(r.tasks[0])
    exc = assert_raise IronWorker::ApiError do
      @titan.groups_name_tasks_id_cancel_post(group_name, r.tasks[0].id)
    end
    assert_equal 409, exc.code
  end

  # Try cancelling failed task
  def test_cancel_error
    group_name = 'cancellation'
    r = @titan.groups_name_tasks_post(group_name, tasks: [{
        image: 'iron/error', 
    }])
    task = wait_for_completion(r.tasks[0])
    assert_equal "error", task.status
    exc = assert_raise IronWorker::ApiError do
      @titan.groups_name_tasks_id_cancel_post(group_name, r.tasks[0].id)
    end
    assert_equal 409, exc.code
  end

  # Try cancelling running task
  def test_cancel_running
    group_name = 'cancellation'
    r = @titan.groups_name_tasks_post(group_name, tasks: [{
      image: 'iron/sleeper:latest',
      payload: {sleep: 10}.to_json,
      }])
    task = wait_for_running(r.tasks[0])
    r = @titan.groups_name_tasks_id_cancel_post(group_name, r.tasks[0].id)
    assert_equal r.task.id, task.id
    assert_equal r.task.status, "cancelled"

    sleep 15
    r = @titan.groups_name_tasks_id_get(group_name, task.id)
    # The task should not transition to success or error
    assert_equal r.task.status, "cancelled"
  end

  # Try cancelling cancelled task
  def test_cancel_cancel
    group_name = 'cancellation'
    r = @titan.groups_name_tasks_post(group_name, tasks: [{
      image: 'iron/sleeper:latest',
      payload: {sleep: 10}.to_json,
      }])
    task = wait_for_running(r.tasks[0])
    r = @titan.groups_name_tasks_id_cancel_post(group_name, task.id)
    assert_equal r.task.id, task.id
    assert_equal r.task.status, "cancelled"

    sleep 15
    r = @titan.groups_name_tasks_id_get(group_name, task.id)
    # The task should not transition to success or error
    assert_equal r.task.status, "cancelled"

    exc = assert_raise IronWorker::ApiError do
      @titan.groups_name_tasks_id_cancel_post(group_name, task.id)
    end
    assert_equal 409, exc.code
  end

  # Try cancelling queued task.
  def test_cancel_queued
    group_name = 'cancellation'
    last_running_task = nil
    last_task = nil
    5.times do |i|
      last_running_task = last_task
      last_task = post_simple_task(group_name).tasks[0]
    end

    r = @titan.groups_name_tasks_id_get(group_name, last_task.id)
    assert_equal r.task.status, "queued"
    r = @titan.groups_name_tasks_id_cancel_post(group_name, last_task.id)
    assert_equal r.task.id, last_task.id
    assert_equal r.task.status, "cancelled"

    task = wait_for_completion(last_running_task)
    assert_equal task.status, "success"
    sleep 2

    r = @titan.groups_name_tasks_id_get(group_name, last_task.id)
    # The task should not transition to success or error, nor should it have
    # a log.
    assert_equal last_task.id, r.task.id
    assert_equal r.task.status, "cancelled"
    puts "cancelled task id: #{r.task.id}"
    exc = assert_raise IronWorker::ApiError do
      @titan.groups_name_tasks_id_log_get(group_name, last_task.id)
    end
    assert_equal 404, exc.code
  end

  # Try cancelling non-existent task
  def test_cancel_non_existent
    group_name = 'cancellation'

    exc = assert_raise IronWorker::ApiError do
      @titan.groups_name_tasks_id_cancel_post(group_name, "-1")
    end
    assert_equal 404, exc.code
  end

  def test_cancel_delayed
    group_name = 'cancellation'
    task = @titan.groups_name_tasks_post(group_name, tasks: [{image: 'iron/hello', payload: {name: "Johnny Utah"}.to_json, delay: 5}]).tasks[0]

    r = @titan.groups_name_tasks_id_get(group_name, task.id)
    assert_equal r.task.status, "delayed"
    r = @titan.groups_name_tasks_id_cancel_post(group_name, task.id)
    assert_equal r.task.id, task.id
    assert_equal r.task.status, "cancelled"

    sleep 10
    r = @titan.groups_name_tasks_id_get(group_name, task.id)
    # The task should not transition to success or error, nor should it have
    # a log.
    assert_equal r.task.status, "cancelled"
    exc = assert_raise IronWorker::ApiError do
      @titan.groups_name_tasks_id_log_get(group_name, task.id)
    end
    assert_equal 404, exc.code
  end
  
  def test_groups
    group_name = 'groups_test'
    g = @titan_groups.groups_name_put(group_name, group: {image: "iron/checker", env_vars: {"FOO"=>"bar", "DB_URL" => "postgres://abc.com/"}, max_concurrency: 10})
    p g
    g2 = @titan_groups.groups_name_get(group_name).group
    assert_equal "bar", g2.env_vars["FOO"]
    task = @titan.groups_name_tasks_post(group_name, tasks: [{payload: {env_vars: {"FOO"=>"bar"}}.to_json}]).tasks[0]
    task = wait_for_completion(task)
    assert_equal task.status, "success"
    
  end  
  
  def test_pagination
      # post 50 tasks, get them in groups of 10 and compare ids/timestamps or something
      group_name = 'paging_test'
      posted_tasks = []
      num_tasks = 25
      num_tasks.times do |i|
        task = @titan.groups_name_tasks_post(group_name, tasks: [{image: 'iron/hello', payload: {name: "Johnny Utah"}.to_json}]).tasks[0]
        puts "posted task id #{task.id}"
        posted_tasks << task        
      end
      
      ji = num_tasks      
      cursor = nil
      n = 5
      (num_tasks/n).times do |i|
        tasksw = @titan.groups_name_tasks_get(group_name, n: n, cursor: cursor)
        tasksw.tasks.each do |task|
          ji -= 1
          puts "got task #{task.id}"
          assert_equal posted_tasks[ji].id, task.id 
        end
        cursor = tasksw.cursor      
        puts "cursor #{cursor}"
        break if cursor == nil   
      end

    wait_for_completion(posted_tasks.last)
  end

  def cancel_tasks(tasks) 
    tasks.each do |j|
      @titan.groups_name_tasks_id_cancel_post(j.group_name, j.id)
    end

  end
  
end
