namespace formatter_smoke {
class Counter { public: explicit Counter(int start): value_(start) {} int add(int step){ value_ += step; return value_; } private: int value_; };
}

int main(){formatter_smoke::Counter counter(40); return counter.add(2)==42 ? 0 : 1;}
