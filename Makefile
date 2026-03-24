.PHONY: default clean
default: clean 
clean:
	rm -f cert.pem key.pem client1.db client2.db server.db
