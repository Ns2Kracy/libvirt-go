#include <stdlib.h>
#include <string.h>

struct parameter {
 char field[80];
 int type;
 union { unsigned long long ul; } value;
};

int virInitialize(void) { return 0; }

char *virDomainGetSchedulerType(void *domain, int *count) {
 *count = 1;
 return strdup("posix");
}

int virDomainGetSchedulerParametersFlags(void *domain, struct parameter *params, int *count, unsigned int flags) {
 if (!params || *count != 1) return -1;
 strcpy(params[0].field, "cpu_shares");
 params[0].type = 4;
 params[0].value.ul = 1024;
 return 0;
}
