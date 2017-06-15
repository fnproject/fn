This is the base image for all docker-in-docker images. 

The difference between this and the official `docker` images are that this will choose the best 
filesystem automatically. The official ones use `vfs` (bad) by default unless you pass in a flag. 
