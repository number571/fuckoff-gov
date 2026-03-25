.PHONY: default install-deps build clean
default: build 
install-deps:
	go install github.com/fyne-io/fyne-cross@latest
build:
	mkdir -p bin
	for arch in amd64 arm64; \
	do \
		for platform in linux windows android; \
		do \
			echo "build $${arch}_$${platform}"; \
			fyne-cross $${platform} -arch=$${arch} --app-id fuckoff.gov.chat --icon images/icons/icon.png; \
			if [[ $$platform == "windows" ]] \
			then \
				cp fyne-cross/bin/$${platform}-$${arch}/fuckoff-gov.exe ./bin/fuckoff-gov_$${arch}_$${platform}.exe; \
			else \
				cp fyne-cross/bin/$${platform}-$${arch}/fuckoff-gov ./bin/fuckoff-gov_$${arch}_$${platform}; \
			fi; \
		done; \
	done;
clean:
	rm -f *.pem *.db
