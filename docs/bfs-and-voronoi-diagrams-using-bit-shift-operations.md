# BFS and Voronoi diagrams using bit-shift operations

> Source: [CodinGame Playground #66330](https://www.codingame.com/playgrounds/66330/bfs-and-voronoi-diagrams-using-bit-shift-operations/introduction) by **wlesavo**

## 1/4 Introduction

Looking for ways to speed up some parts of the search within a game states one would often hear about a mystical "bitshifts" and "bitboards". I found myself in this position and fragmentary knowledge i was able to find in internet did not help much. Maybe there are these types of articles but i just couldn't find one to help me solve even a quite simple tasks. Although we do have some excelent playgrounds from MSmits and emh which were a starting point for me, but for the rest for many things I would have to come to mind by myself. I highly recommend checking these playgrounds prior to this article, since we will not go into much details about the common BFS itself.

Wrapping up, in this playground I will try to demonstrate a simple way to think about BFS using bitboards on example of three different CG games: Great Escape, Amazonial and Line Racing going for somewhat diverse goals but showcasing the same idea. In Great Escape we will find the length of a shortest path, which is a main heuristic in evauation function for this game. In Amazonial and Line Racing we will calculate the Voronoi diagramms (number of cells one player can reach faster than another and vise versa), which are also the main heuristics for those games while being the most time consuming operations in simulation. The difference for a Line Racing is having up to 4 players and the fact that the board can't be reasonably stored in a single integer value since it has 600 cells.

I would also encourage more experienced users to point out to any mistakes I made or a bad practicies I had accidentally used.

### Related playgrounds

- [Fast 6x12 Connected Components using bit-optimized BFS](https://www.codingame.com/playgrounds/93367) by emh
- [Fast Connected Components for 6x12 bitboard](https://www.codingame.com/playgrounds/90507) by emh
- [Optimizing breadth first search](https://www.codingame.com/playgrounds/38626/optimizing-breadth-first-search) by MSmits
- [Breadth First Search and Beam Search Comparison](https://www.codingame.com/playgrounds/61830) by Unnamed contributor

---

## 2/4 Shortest path in Great Escape

### Coordinates and connections on a grid

A common way to store a 2-dimensional array is to put it in 1-dimensional array performing a simple transformation for coordinates:

```cpp
int pos = x + y * WIDTH;
```

with a back transformation:

```cpp
int x = pos % WIDTH;
int y = pos / WIDTH;
```

To move position say UP, we would have to do it the hard way:

```cpp
int x = pos % WIDTH;
int y = pos / WIDTH;
int new_x = x;
int new_y = y - 1;
// perform all necessary out of bounds checks
int new_pos = new_x + new_y * WIDTH;
```

But there is also an equivalent unsafe method to do the same:

```cpp
int new_pos = pos - WIDTH;
```

This is quite faster, but we have to make sure that an impossible moves will be rejected.

Lets move on. To perform a simple BFS we will store connections for each direction in a single `uint128_t` value. Set bit for connection would mean that it is possible to walk from current possition to a corresponding direction.

```cpp
class State {
public:
    uint128_t con[4] = {};
    int player_pos = pos;
};
```

To set or reset bit we will use a simple well known functions:

```cpp
template <class T>
static inline void set_bit(T& val, int n) {
    val |= (T)1 << n;
}

template <class T>
static inline void reset_bit(T& val, int n) {
    val &= ~((T)1 << n);
}
```

Preparing the state is described in the executed code example, although during the search we may simply turn on and off the connections needed, so that this connections will always be in an actualized state.

### Performing bit-shift BFS for finding the shortest path length

To perform a BFS we will need a containers to store current positions along with all visited positions. To move all current possitions in some direction (for example UP) we perform 'and' and 'shift' operations.

```cpp
(cur & con[d]) >> WIDTH;
```

Here 'and' opperation resets all currently stored positions to 0 if the move in chosen direction is impossible, making it ready to use the 'unsafe' move, described above. This way moving simultaneously in all directions is performed by simply merging all the moves for URDL with 'or'. We also will have to keep track of all visits to not go back to visited cells. Basically we are done, a simple BFS then will look like this:

```cpp
int BFS(const State& s) {
    uint128_t cur = 0;
    set_bit(cur, s.player_pos);
    uint128_t visits = cur;
    int dist = 0;
    while (true) {
        if (cur == 0) {
            return dist;
        }
        cur = (cur & s.con[0]) >> WIDTH
            | (cur & s.con[1]) << 1
            | (cur & s.con[2]) << WIDTH
            | (cur & s.con[3]) >> 1;
        cur &= ~visits;
        visits |= cur;
        dist += 1;
    }
    return -1;
}
```

The attentive reader would notice that here BFS misses win condition, so it performs a full scout until all reachable cells are visited. We will further fix this in the final executable example.

### Code example

The full executable example demonstrates connections preparation and win masks calculation for a 9x9 Great Escape board using `uint128_t`:

```cpp
#include <immintrin.h>
#include <bits/stdc++.h>
using namespace std;

#define uint128_t __int128

const int WIDTH = 9;
const int HEIGHT = 9;
const int SIZE = 9 * 9;

template <class T>
static inline void set_bit(T& val, int n) {
    val |= (T)1 << n;
}

template <class T>
static inline void reset_bit(T& val, int n) {
    val &= ~((T)1 << n);
}

template <class T>
static inline bool check_bit(const T& a, int n) {
    return (a >> n) & 1;
}

class State {
public:
    uint128_t con[4] = {};
    int player_pos[3] = {};

    State() {};

    void set_wall(int pos, int type) {
        if (type == 0) {
            reset_bit(con[2], pos);
            reset_bit(con[2], pos + 1);
            reset_bit(con[0], pos + WIDTH);
            reset_bit(con[0], pos + WIDTH + 1);
        } else {
            reset_bit(con[1], pos);
            reset_bit(con[1], pos + WIDTH);
            reset_bit(con[3], pos + 1);
            reset_bit(con[3], pos + WIDTH + 1);
        }
    }
};
```

```cpp
//{
#pragma GCC optimize("O3,inline,omit-frame-pointer,unroll-loops","unsafe-math-optimizations","no-trapping-math")
#pragma GCC option("arch=native","tune=native","no-zero-upper")
#pragma GCC target("sse,sse2,sse3,ssse3,sse4,mmx,avx,avx2,popcnt,rdrnd,abm,bmi2,fma")
#ifdef __GNUC__
#include <cfenv>
#endif
#include <iostream>
#include <chrono>
//}

#include <immintrin.h>
#include <bits/stdc++.h>
using namespace std;
using namespace std::chrono;
#define uint128_t __int128

//{
    
const int WIDTH = 9;
const int HEIGHT = 9;
const int SIZE = 9 * 9;

template <class T>
static inline void set_bit(T& val, int n){
  val |= (T)1 << n;
}
template <class T>
static inline void reset_bit(T& val, int n){
  val &= ~((T)1 << n);
}
template <class T>
static inline bool check_bit(const T& a, int n){
  return (a >> n) & 1;
}

class State {
public:  
  uint128_t con[4] = {};
  int player_pos[3] = {};
  State() {};
  void set_wall(int pos, int type){
    if (type == 0){
      reset_bit(con[2], pos);
      reset_bit(con[2], pos+1);
      reset_bit(con[0], pos+WIDTH);
      reset_bit(con[0], pos+WIDTH+1);
    }
    else{
      reset_bit(con[1], pos);
      reset_bit(con[1], pos+WIDTH);
      reset_bit(con[3], pos+1);
      reset_bit(con[3], pos+1+WIDTH);
    }
  }

};
//}

class Game {
public:

  State cur_state{};
  uint128_t player_won[3] = {};
  int bfs_rolls = 0;
  Game() {
    // activate all connections
    for (int i = 0; i < 4; i++){
      cur_state.con[i] = ((uint128_t)1 << 81) - (uint128_t)1;
    }
    // remove boundary connections
    for (int i = 0; i < WIDTH; i++) {
      reset_bit(cur_state.con[0], i);
      reset_bit(cur_state.con[1], i * WIDTH + WIDTH - 1);
      reset_bit(cur_state.con[2], i + WIDTH * (HEIGHT - 1));
      reset_bit(cur_state.con[3], i * WIDTH);
    }
    
    // prepare win-condition masks
    for (int i = 0; i < WIDTH; i++){     
      set_bit(player_won[0],(i*WIDTH + HEIGHT - 1));
      set_bit(player_won[1], i*WIDTH);
      set_bit(player_won[2], (HEIGHT - 1)*HEIGHT + i);      
    }
    
  }
  
  int get_path_len(State& s, int owner){
    int pos = s.player_pos[owner];
    if (check_bit(player_won[owner], pos)){
      return 0;
    }
    bfs_rolls += 1;
    uint128_t win = player_won[owner];
    uint128_t cur = (uint128_t)1<<pos;
    uint128_t visits = cur;
    int dist = 0;
    while (true) {
      if (cur==0){
        return -1;
      }
      if ((cur&win)>0){
        return dist;
      }
      cur = (cur&s.con[0])>>WIDTH | (cur&s.con[1])<<1 | (cur&s.con[2])<<WIDTH | (cur&s.con[3])>>1;
      cur &= ~visits;
      visits |= cur;
      dist += 1;
    }
    return -1;    
  }

  void init_cur_state(){
    //init player position
    cur_state.player_pos[0] = 2 + 4 * WIDTH;
    //add some vertical walls
    cur_state.set_wall(3+3*WIDTH, 1);
    cur_state.set_wall(4+4*WIDTH, 1);
  }
  
  void test() {
    system("cat /proc/cpuinfo | grep \"model name\" | head -1 >&2");
    system("cat /proc/cpuinfo | grep \"cpu MHz\" | head -1 >&2");

    ios::sync_with_stdio(false);
#ifdef __GNUC__
    feenableexcept(FE_DIVBYZERO | FE_INVALID | FE_OVERFLOW);
#endif

    init_cur_state();
    int total_dist = 0;
    int roll_count = 1000000;
    auto start = high_resolution_clock::now();
    for (int i = 0; i<roll_count; i++){
        total_dist += get_path_len(cur_state, 0);
    }
    auto end = high_resolution_clock::now();
    auto full_count = duration_cast<microseconds>(end - start).count()/1000;
    cerr << "total execution time " << full_count << "ms with " << bfs_rolls << " rolls"<<  endl;
    cerr << "calculated min distance " << total_dist/bfs_rolls << endl;
  }
};

int main()
{
  Game g = Game();
  g.test();
}
```

---

## 3/4 Voronoi area in Amazonial

As was previously said, we will now try to apply similar concept to solve a quite different task. Namely we will calculate the number of cells one player can reach faster than another and vise versa, which is a special case of Voronoi diagrams calculated on a grid.

### General idea

The game Amazonial is not that different from the Great Escape if you look at it at the right angle. Although it still requires quite a few modifications, i.e accounting for multiple units and 8 available dirrections. The later will lead to a modification of our BFS as:

```cpp
cur = (cur & con[0]) >> WIDTH       | (cur & con[1]) >> (WIDTH - 1)
    | (cur & con[2]) << 1           | (cur & con[3]) << (WIDTH + 1)
    | (cur & con[4]) << WIDTH       | (cur & con[5]) << (WIDTH - 1)
    | (cur & con[6]) >> 1           | (cur & con[7]) >> (WIDTH + 1);
```

The rest is simple. We first calculate new positions on a current depth for both players, by expanding current cells and substracting all the cells already visited by any player. Then the cells that only one player reached on a current depth are added to the positions that has been won. Then we finally update visited cells. After the BFSs for both players are finished, we calculate the area by counting set bits with a simple recursive function.

```cpp
template <class T>
static int count_set_bits(T n) {
    if (n == 0)
        return 0;
    else
        return (n & 1) + count_set_bits(n >> 1);
}
```

### Voronoi calculation

```cpp
void voronoi(const State& s) {
    // set starting positions for both units
    for (int i = 0; i < 2; i++) {
        int pos1 = s.players[0].pos[i];
        set_bit(cur1, pos1);
        int pos2 = s.players[1].pos[i];
        set_bit(cur2, pos2);
    }
    uint64_t my_cells = cur1;
    uint64_t opp_cells = cur2;
    uint64_t visited = cur1 | cur2;

    while (cur1 != 0 || cur2 != 0) {
        cur1 = (cur1 & s.con[0]) >> WIDTH       | (cur1 & s.con[1]) >> (WIDTH - 1)
             | (cur1 & s.con[2]) << 1           | (cur1 & s.con[3]) << (WIDTH + 1)
             | (cur1 & s.con[4]) << WIDTH       | (cur1 & s.con[5]) << (WIDTH - 1)
             | (cur1 & s.con[6]) >> 1           | (cur1 & s.con[7]) >> (WIDTH + 1);
        cur1 &= ~visited;

        cur2 = (cur2 & s.con[0]) >> WIDTH       | (cur2 & s.con[1]) >> (WIDTH - 1)
             | (cur2 & s.con[2]) << 1           | (cur2 & s.con[3]) << (WIDTH + 1)
             | (cur2 & s.con[4]) << WIDTH       | (cur2 & s.con[5]) << (WIDTH - 1)
             | (cur2 & s.con[6]) >> 1           | (cur2 & s.con[7]) >> (WIDTH + 1);
        cur2 &= ~visited;

        my_cells  |= (cur1 & (~cur2));
        opp_cells |= (cur2 & (~cur1));

        visited |= cur1 | cur2;
    }

    int my_score  = count_set_bits(my_cells);
    int opp_score = count_set_bits(opp_cells);
}
```

### Code example

The full executable example demonstrates the connections preparation for an 8x8 Amazonial board using `uint64_t` with 8 directions.

```cpp
//{
#pragma GCC optimize("O3,inline,omit-frame-pointer,unroll-loops","unsafe-math-optimizations","no-trapping-math")
#pragma GCC option("arch=native","tune=native","no-zero-upper")
#pragma GCC target("sse,sse2,sse3,ssse3,sse4,mmx,avx,avx2,popcnt,rdrnd,abm,bmi2,fma")
#ifdef __GNUC__
#include <cfenv>
#endif
#include <iostream>
#include <chrono>
using namespace std;
using namespace std::chrono;
    
const int WIDTH = 8;
const int HEIGHT = 8;
const int SIZE = 8 * 8;

template <class T>
static int count_set_bits(T n)
{
  if (n == 0)
    return 0;
  else
    return (n & 1) + count_set_bits(n >> 1);
}
template <class T>
static inline void set_bit(T& val, int n){
  val |= (T)1 << n;
}
template <class T>
static inline void reset_bit(T& val, int n){
  val &= ~((T)1 << n);
}
template <class T>
static inline bool check_bit(const T& a, int n){
  return (a >> n) & 1;
}

class State {
public:  
  uint64_t con[8] = {};
  int player_pos[2][2] = {};
  int voronoi_score[2] = {};
  State() {};
};
//}

class Game {
public:

  State cur_state{};
  int voronoi_rolls = 0;
  Game() {
    // activate all connections
    for (int i = 0; i < 4; i++){
      cur_state.con[i] = (uint64_t)(-1);
    }
    // remove boundary connections
    for (int i = 0; i < WIDTH; i++) {
      reset_bit(cur_state.con[0], i);
      reset_bit(cur_state.con[1], i);
      reset_bit(cur_state.con[1], i * WIDTH + WIDTH - 1);
      reset_bit(cur_state.con[2], i * WIDTH + WIDTH - 1);
      reset_bit(cur_state.con[3], i * WIDTH + WIDTH - 1);
      reset_bit(cur_state.con[3], i + WIDTH * (HEIGHT - 1));
      reset_bit(cur_state.con[4], i + WIDTH * (HEIGHT - 1));      
      reset_bit(cur_state.con[5], i + WIDTH * (HEIGHT - 1));
      reset_bit(cur_state.con[5], i * WIDTH);
      reset_bit(cur_state.con[6], i * WIDTH);
      reset_bit(cur_state.con[7], i * WIDTH);
      reset_bit(cur_state.con[7], i);
    }    
  }
  
  void voronoi(State& s){
  // set strating positions for both units
    uint64_t cur1 = 0;
    uint64_t cur2 = 0;
    for (int i = 0; i<2; i++){
      int pos1 = s.player_pos[0][i];
      set_bit(cur1, pos1);
      int pos2 = s.player_pos[1][i];
      set_bit(cur2, pos2);
    }
    uint64_t my_cells = cur1;
    uint64_t opp_cells = cur2;
    uint64_t visited = cur1 | cur2;
    voronoi_rolls += 1;
    
    while (cur1!=0 || cur2 != 0) {
      cur1 = (cur1 & s.con[0]) >> WIDTH | (cur1 & s.con[1]) >> (WIDTH - 1) 
           | (cur1 & s.con[2]) << 1     | (cur1 & s.con[3]) << (WIDTH + 1) 
           | (cur1 & s.con[4]) << WIDTH | (cur1 & s.con[5]) << (WIDTH - 1) 
           | (cur1 & s.con[6]) >> 1     | (cur1 & s.con[7]) >> (WIDTH + 1);
      cur1 &= ~visited;
    
      cur2 = (cur2 & s.con[0]) >> WIDTH | (cur2 & s.con[1]) >> (WIDTH - 1) 
           | (cur2 & s.con[2]) << 1     | (cur2 & s.con[3]) << (WIDTH + 1) 
           | (cur2 & s.con[4]) << WIDTH | (cur2 & s.con[5]) << (WIDTH - 1) 
           | (cur2 & s.con[6]) >> 1     | (cur2 & s.con[7]) >> (WIDTH + 1);
      cur2 &= ~visited;
      
      my_cells  |= (cur1 & (~cur2));
      opp_cells |= (cur2 & (~cur1));
    
      visited |= cur1 | cur2;
    }
    s.voronoi_score[0] = count_set_bits(my_cells);
    s.voronoi_score[1] = count_set_bits(opp_cells);
}

  void init_cur_state(){
    //init player position
    cur_state.player_pos[0][0] = 2 + 4 * WIDTH;
    cur_state.player_pos[0][1] = 3 + 5 * WIDTH;
    cur_state.player_pos[1][0] = 1 + 2 * WIDTH;
    cur_state.player_pos[1][1] = 4 + 6 * WIDTH;
    
  }
  
  void test() {
    system("cat /proc/cpuinfo | grep \"model name\" | head -1 >&2");
    system("cat /proc/cpuinfo | grep \"cpu MHz\" | head -1 >&2");

    ios::sync_with_stdio(false);
#ifdef __GNUC__
    feenableexcept(FE_DIVBYZERO | FE_INVALID | FE_OVERFLOW);
#endif

    init_cur_state();
    int total_dist = 0;
    int roll_count = 1000000;
    auto start = high_resolution_clock::now();
    for (int i = 0; i<roll_count; i++){
        voronoi(cur_state);
    }
    auto end = high_resolution_clock::now();
    auto full_count = duration_cast<microseconds>(end - start).count()/1000;
    cerr << "total execution time " << full_count << "ms with " << voronoi_rolls << " rolls"<<  endl;
    cerr << "calculated voronoi score player 1: " << cur_state.voronoi_score[0] << " player 2: " << cur_state.voronoi_score[1] << endl;
  }
};

int main()
{
  Game g = Game();
  g.test();
}
```

---

## 4/4 Voronoi area in Line Racing

Line Racing (aka Tron) is a game with quite simple rules and simulation, although it does heavily relies on a Voronoi diagrams calculation. For quite some time now I thought to myself of how to implement the algortihm similar to the previous part on such a large board. I ended up writing my own class storring 5 `uint128_t` values with a shift operations looking like this:

```cpp
struct Word5 {
public:
    uint128_t val[5] = {};
};

Word5 shift_bits_right(int n) {
    Word5 out{};
    const int rev_n = INT_SIZE - n;
    out.val[4] = (val[4] >> n) | (val[3] << (rev_n));
    out.val[3] = (val[3] >> n) | (val[2] << (rev_n));
    out.val[2] = (val[2] >> n) | (val[1] << (rev_n));
    out.val[1] = (val[1] >> n) | (val[0] << (rev_n));
    out.val[0] = out.val[0] >> n;
    return out;
}
```

Although it did actually worked, giving the same result as the vanila queue implementation, the speed up was only up to 3-4 times. Comming to create this playground, I realised, that C++ `bitset<>` allows to perform all logical operations I need. Eventually bitset implementation had close to exactly the same speed as my own implementation, laking the pain of writing all of the support functions. So I decided to demonstrate only the bitset version, although I would really like to hear if there are any more effective alternatives. Appart from bitsets this version differs from Amazonial only in minor details, so without further ado here is an executable example.

### Code example (bitset version for 30x20 board with 4 players)

```cpp
#include <bitset>
using namespace std;

const int WIDTH = 30;
const int HEIGHT = 20;
const int SIZE = WIDTH * HEIGHT;

class State {
public:
    bitset<SIZE> con[4] = {};
    int player_pos[4] = {};
    int voronoi_score[4] = {};

    State() {};
};

class Game {
public:
    State cur_state{};

    Game() {
        // activate all connections
        for (int i = 0; i < 4; i++) {
            cur_state.con[i] = ~(cur_state.con[i]);
        }
        // remove boundary connections
        for (int i = 0; i < WIDTH; i++) {
            cur_state.con[0].reset(i);
            cur_state.con[2].reset(i + WIDTH * (HEIGHT - 1));
        }
        for (int i = 0; i < HEIGHT; i++) {
            cur_state.con[1].reset(i * WIDTH + WIDTH - 1);
            cur_state.con[3].reset(i * WIDTH);
        }
    }

    void voronoi(State& s) {
        // set starting positions for all players
        bitset<SIZE> cur[4] = {};
        for (int p = 0; p < 4; p++) {
            cur[p].set(s.player_pos[p]);
        }
        // ... BFS expansion similar to Amazonial but for 4 players
        // using bitset shift: (cur[p] & con[0]) >> WIDTH, etc.
    }
};
```

```cpp
//{
#pragma GCC optimize("O3,inline,omit-frame-pointer,unroll-loops","unsafe-math-optimizations","no-trapping-math")
#pragma GCC option("arch=native","tune=native","no-zero-upper")
#pragma GCC target("sse,sse2,sse3,ssse3,sse4,mmx,avx,avx2,popcnt,rdrnd,abm,bmi2,fma")
#ifdef __GNUC__
#include <cfenv>
#endif
#include <iostream>
#include <chrono>
#include <bitset>
using namespace std;
using namespace std::chrono;

const int WIDTH = 30;
const int HEIGHT = 20;
const int SIZE = WIDTH * HEIGHT;

class State {
public:  
  bitset<SIZE> con[4] = {};
  int player_pos[4] = {};
  int voronoi_score[4] = {};
  State() {};
};
//}

class Game {
public:

  State cur_state{};
  int voronoi_rolls = 0;
  Game() {
    // activate all connections
    for (int i = 0; i < 4; i++){
      cur_state.con[i] = ~(cur_state.con[i]);
    }
    // remove boundary connections
    for (int i = 0; i < WIDTH; i++) {
      cur_state.con[0].reset(i);
      cur_state.con[2].reset(i + WIDTH * (HEIGHT - 1));
    }
    for (int i = 0; i < HEIGHT; i++) {
      cur_state.con[1].reset(i * WIDTH + WIDTH - 1);
      cur_state.con[3].reset(i * WIDTH);
    }
  }
  
  void voronoi(State& s){
    // set strating positions for both units
    bitset<SIZE> cur[4] = {};
    for (int p = 0; p < 4; p++) {
      cur[p].set(s.player_pos[p]);
    }
    voronoi_rolls += 1;
    bitset<SIZE> owned_cells[4] = {};
    bitset<SIZE> visits{};
    for (int p = 0; p < 4; p++) {
      owned_cells[p] = cur[p];
      visits |= cur[p];
    }
    
    int dist = 0;
    int score = 0;
    const int w = 30;
    bool skip[4] = {};
    //*
    while (true) {
      for (int i = 0; i<4; i++){
        if (skip[i])
          continue;
        cur[i] = (cur[i]&s.con[0])>>w
               | (cur[i]&s.con[1])<<1
               | (cur[i]&s.con[2])<<w
               | (cur[i]&s.con[3])>>1;
        cur[i] &= ~visits;
      }
      dist += 1;
      bitset<SIZE> opp_total[4]{};
      for (int i = 0; i<4; i++){
        if (skip[i])
          continue;
        for (int j = 0; j<4; j++){
          if (i!=j)
            opp_total[i] |= cur[j];
        }
      }
      for (int i = 0; i<4; i++){
        if (skip[i])
          continue;
        owned_cells[i]|=cur[i]&(~opp_total[i]);
      }
      bool flag = true;
      for (int i = 0; i<4; i++){
        skip[i] = !(cur[i].any());
        flag = flag && skip[i];
      }
      if (flag){
        break;
      }
      for (int i = 0; i<4; i++){
        visits |= cur[i];
      }
    }
    for (int i = 0; i<4; i++){
      s.voronoi_score[i] = owned_cells[i].count();
    }
  }

  void init_cur_state(){
    //init player position
    cur_state.player_pos[0] = 1 + 5 * WIDTH;
    cur_state.player_pos[1] = 10 + 15 * WIDTH;
    cur_state.player_pos[2] = 4 + 14 * WIDTH;
    cur_state.player_pos[3] = 23 + 17 * WIDTH;
    
  }
  
  void test() {
    system("cat /proc/cpuinfo | grep \"model name\" | head -1 >&2");
    system("cat /proc/cpuinfo | grep \"cpu MHz\" | head -1 >&2");

    ios::sync_with_stdio(false);
#ifdef __GNUC__
    feenableexcept(FE_DIVBYZERO | FE_INVALID | FE_OVERFLOW);
#endif

    init_cur_state();
    int total_dist = 0;
    int roll_count = 5000;
    auto start = high_resolution_clock::now();
    for (int i = 0; i<roll_count; i++){
        voronoi(cur_state);
    }
    auto end = high_resolution_clock::now();
    auto full_count = duration_cast<microseconds>(end - start).count()/1000;
    cerr << "total execution time " << full_count << "ms with " << voronoi_rolls << " rolls" <<  endl;
    cerr << "calculated voronoi score player 1: " << cur_state.voronoi_score[0] 
         << " player 2: " << cur_state.voronoi_score[1]
         << " player 3: " << cur_state.voronoi_score[2]
         << " player 4: " << cur_state.voronoi_score[3] << endl;
  }
};

int main()
{
  Game g = Game();
  g.test();
}
```

---

## Key Takeaways

1. **Bitboard BFS**: Store grid connections as bitmasks. Moving all positions in a direction is a single AND + SHIFT operation.
2. **Voronoi on bitboard**: Expand both players' BFS simultaneously. Cells reached by only one player belong to that player's Voronoi area.
3. **Scaling beyond 128 bits**: For large boards (e.g. 30x20 = 600 cells), use `bitset<N>` or custom multi-word integers.
4. **Direction encoding** (4-directional):
   - UP: `(cur & con[0]) >> WIDTH`
   - RIGHT: `(cur & con[1]) << 1`
   - DOWN: `(cur & con[2]) << WIDTH`
   - LEFT: `(cur & con[3]) >> 1`
5. **Connection bitmasks** act as guards — AND-ing before shifting prevents wrapping across boundaries.
