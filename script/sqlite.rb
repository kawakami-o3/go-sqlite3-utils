#!/usr/bin/env ruby
p ARGV
file = ARGV.shift

ARGV.each do |sql|
	puts "echo '#{sql}' | sqlite3 #{file}"
	system "echo '#{sql}' | sqlite3 #{file}"
end

