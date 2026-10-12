/* Owned fixture: exec from a mapped store file to a store executable. */
#define _GNU_SOURCE
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <unistd.h>

int main(int argc, char **argv) {
    if (argc == 5 && !strcmp(argv[4], "after")) {
        char execd[4096];
        snprintf(execd, sizeof execd, "%s/execd", argv[1]);
        int marker = open(execd, O_WRONLY | O_CREAT | O_CLOEXEC, 0600);
        if (marker < 0 || close(marker)) return 96;
        for (;;) pause();
    }
    if (argc != 4) return 90;
    int fd = open(argv[2], O_RDONLY | O_CLOEXEC);
    if (fd < 0) return 91;
    if (mmap(0, 4096, PROT_READ, MAP_PRIVATE, fd, 0) == MAP_FAILED) return 92;
    close(fd);
    for (int i = 0; i < 200; i++) {
        if (mmap(0, 4096, (i % 2) ? PROT_READ : PROT_READ | PROT_WRITE, MAP_PRIVATE | MAP_ANONYMOUS, -1, 0) == MAP_FAILED) return 93;
    }
    char ready[4096];
    snprintf(ready, sizeof ready, "%s/ready", argv[1]);
    int marker = open(ready, O_WRONLY | O_CREAT | O_CLOEXEC, 0600);
    if (marker < 0 || close(marker)) return 94;
    char trigger[4096];
    if (snprintf(trigger, sizeof trigger, "%s/exec", argv[1]) >= (int)sizeof trigger) return 97;
    int attempts = 0;
    while (access(trigger, F_OK)) { if (++attempts > 5000) return 98; usleep(1000); }
    char *next[] = {argv[3], argv[1], argv[2], argv[3], "after", 0};
    execv(argv[3], next);
    return 95;
}
