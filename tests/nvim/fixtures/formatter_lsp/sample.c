struct Counter { int value; };

static int counter_add(struct Counter *counter,int step){counter->value+=step;return counter->value;}

int main(void){struct Counter counter={40};return counter_add(&counter,2)==42?0:1;}
