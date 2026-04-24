# CC Assignment 3 — SLR(1) & LR(1) Parser
## Report

**Course:** Compiler Construction  
**Team Members:**  
- Ibrahim Azhar — 22i-0928  
- Ayan Asif — 22i-1097  

**Language:** C++14  
**Date:** April 2026

---

## 1. Introduction

### Bottom-Up Parsing

Bottom-up parsing is a strategy for analysing a string of tokens by starting at the leaves (terminals) and working toward the root (start symbol) of the parse tree. The parser repeatedly identifies a handle — a substring of the current sentential form that matches the right-hand side of some production — and replaces it with the corresponding left-hand side. This process is called a **reduction**, and it continues until the entire input is reduced to the start symbol or an error is detected.

The dominant class of bottom-up parsers is the **LR family**. An LR parser reads input **L**eft-to-right and produces a **R**ightmost derivation in reverse. It uses:

- A **stack** to record the parse history (states and grammar symbols).
- An **ACTION table** that decides whether to *shift* the next input token onto the stack, *reduce* using a production, *accept* the input, or report an *error*.
- A **GOTO table** that drives transitions on non-terminals after a reduction.

### SLR(1) vs LR(1)

Two variants are implemented in this project:

| Feature | SLR(1) | LR(1) |
|---------|--------|-------|
| Items used | LR(0) items | LR(1) items (with lookahead) |
| Reduce decision | FOLLOW(A) for all reductions | Specific lookahead embedded in each item |
| Parsing power | Weaker — more grammars cause conflicts | Stronger — handles strictly more grammars |
| States generated | Fewer (shared across contexts) | More (context is split by lookahead) |

SLR(1) is simpler to build but may report spurious conflicts for grammars that a full LR(1) parser can handle without ambiguity. LR(1) carries precise context (the exact lookahead that triggers each reduction), which resolves these conflicts at the cost of a larger automaton.

---

## 2. Approach

### 2.1 Data Structures

#### Grammar Representation

```
struct Production {
    string lhs;          // left-hand side non-terminal
    vector<string> rhs;  // right-hand side tokens; empty = epsilon
};

class Grammar {
    vector<Production>                productions;
    string                            startSymbol;
    set<string>                       terminals, nonTerminals;
    map<string, set<string>>          firstSets, followSets;
};
```

- **Non-terminal rule:** a symbol whose name has length > 1 and starts with an uppercase letter.  
- **Epsilon** is represented as an empty `rhs` vector; the tokens `epsilon` and `@` in the input file are both treated as epsilon.
- The augmented production `S'→ S` is always stored at index 0, which the table builders rely on for the accept action.

#### LR(0) Items

```
struct LR0Item {
    int prodIdx;   // index into Grammar::productions
    int dotPos;    // 0 … rhs.size(); dotPos == rhs.size() means dot at end
};
using LR0State = set<LR0Item>;
```

A **state** is a set of LR(0) items, stored in a `std::set` so that two states with identical items compare equal. The canonical collection is a `vector<LR0State>` with a parallel `map<string,int>` from a canonical key string to the state index, used for O(1) duplicate detection.

#### LR(1) Items

```
struct LR1Item {
    int    prodIdx;
    int    dotPos;
    string lookahead;   // single terminal or $
};
using LR1State = set<LR1Item>;
```

Each LR(0) item is split into potentially many LR(1) items, one per valid lookahead symbol.

#### Parsing Table

```
enum ActionType { ACT_ERROR, ACT_SHIFT, ACT_REDUCE, ACT_ACCEPT };

struct Action { ActionType type; int value; };

class ParsingTable {
    map<int, map<string, Action>> action;     // ACTION[state][terminal]
    map<int, map<string, int>>    gotoTable;  // GOTO[state][nonterminal]
    vector<Conflict>              conflicts;
};
```

Inserting into `action` via `setAction()` checks whether an entry already exists; if so, a `Conflict` record is appended and the shift is preferred over the reduce (standard precedence heuristic).

#### Parser Stack

A generic template class (in `stack.h`) wraps `std::vector` to provide a typed stack interface used by the parsing engine:

```
template<typename T>
class ParserStack {
    vector<T> data_;
public:
    void push(T), pop(), T& top(), T& operator[](size_t), size_t size(), ...
};
```

Three parallel stacks are maintained during parsing: `ParserStack<int>` (states), `ParserStack<string>` (symbols), and `ParserStack<TreeNode*>` (parse tree nodes).

### 2.2 Algorithm Implementation

#### FIRST Sets

Computed iteratively until a fixed point:

```
for each production A → X1 X2 … Xk:
    for i = 1 to k:
        add FIRST(Xi) − {ε} to FIRST(A)
        if ε ∉ FIRST(Xi): break
    if all Xi can derive ε: add ε to FIRST(A)
```

`firstOfSequence(seq, start)` generalises this to an arbitrary sequence, needed for FOLLOW and LR(1) closure.

#### FOLLOW Sets

Also a fixed-point iteration:

```
FOLLOW(S') = {$}
for each production A → αBβ:
    add FIRST(β) − {ε} to FOLLOW(B)
    if ε ∈ FIRST(β): add FOLLOW(A) to FOLLOW(B)
```

#### LR(0) CLOSURE

```
CLOSURE(I):
    result = I
    repeat:
        for each item [A → α•Bβ] in result:
            for each production B → γ:
                add [B → •γ] to result if not already present
    until no change
```

Implemented with a `bool changed` flag; the inner loop uses a separate `additions` set to avoid modifying the container while iterating.

#### LR(0) GOTO

```
GOTO(I, X):
    moved = { [A → αX•β] | [A → α•Xβ] ∈ I }
    return CLOSURE(moved)
```

#### Building the LR(0) Canonical Collection

A BFS over item sets using a worklist queue. Each state is hashed to a string key before insertion to detect duplicates in O(1).

#### LR(1) CLOSURE

The key difference from LR(0): when adding `[B → •γ, b]`, the lookahead `b` is drawn from `FIRST(β · a)` where `β` is the tail of the current item's production and `a` is the current item's lookahead.

```
for each [A → α•Bβ, a] in result:
    for each production B → γ:
        for each b in FIRST(β·a):        // firstWithLookahead helper
            add [B → •γ, b] if not present
```

`firstWithLookahead(seq, start, la)` computes FIRST of `seq[start..] · la` without materialising the concatenation:

```
iterate through seq[start..]:
    if terminal t: return {t}
    if nonterminal N: add FIRST(N)−{ε}; if ε∉FIRST(N): return
if all derived ε: return {la}    // use the lookahead
```

#### SLR(1) Table Construction

For each state `i` and each item in that state:

- **Shift:** if `A→α•aβ` and `GOTO(i,a)=j` and `a` is a terminal → `ACTION[i,a] = shift j`
- **Reduce:** if `A→α•` (dot at end, A ≠ S') → for each `a ∈ FOLLOW(A)`: `ACTION[i,a] = reduce`
- **Accept:** if `S'→S•` → `ACTION[i,$] = accept`
- **GOTO:** if `GOTO(i,A)=j` and `A` is a non-terminal → `GOTO[i,A] = j`

#### LR(1) Table Construction

Identical structure, but reduce actions are placed only on the specific lookahead `a` stored in the item `[A→α•, a]`, not on the entire FOLLOW set. This is the crucial difference that resolves SLR(1) conflicts.

#### Shift-Reduce Parsing Engine

```
stateStack = [0];  symStack = [];  ip = 0
loop:
    s = stateStack.top();  a = input[ip]
    switch ACTION[s][a]:
        shift j  → push a onto symStack; push j onto stateStack; ip++
        reduce A→β (|β|=n) →
            pop n from both stacks
            t = stateStack.top()
            push A onto symStack; push GOTO[t][A] onto stateStack
        accept   → success
        error    → report and halt
```

At each step, the current stack display, remaining input, and action are printed.

### 2.3 Design Decisions and Trade-offs

| Decision | Rationale |
|----------|-----------|
| `std::set<LR0Item>` for states | Automatic ordering and O(log n) lookup; equality check is just set comparison. |
| String key for state deduplication | Converting a set to a canonical string is O(n log n) but avoids implementing a custom hash for sets. |
| Separate `LR0Collection` / `LR1Collection` structs that carry both states and transitions | The table builder needs transition lookup; returning both avoids recomputing GOTO during table construction. |
| `template<T> ParserStack<T>` | Reusable for state stack, symbol stack, and tree-node stack with no code duplication. |
| Prefer shift in conflict resolution | Industry-standard default (matches most language specifications for dangling-else etc.); the conflict is still reported to the user. |
| All output written to both `stdout` and `output/*.txt` | Enables live feedback during interactive use while preserving results for report screenshots. |

### 2.4 Handling LR(1) Lookaheads

When the LR(1) closure processes an item `[A → α•Bβ, a]`:

1. The **tail** `β` following `B` may derive epsilon, in which case the lookahead `a` from the surrounding context propagates into `B`'s items.
2. Otherwise only the terminals reachable from `β` become lookaheads.

This is implemented in `firstWithLookahead(seq, start, la)` which avoids ever constructing the concatenated sequence `β·a`. The function walks `seq[start..]` exactly as `firstOfSequence` does, but instead of inserting `ε` when the whole tail can derive epsilon, it inserts the lookahead `la`.

The result is that each `[B → •γ, b]` item carries a **precise** lookahead that says "only reduce when the next token is `b`", avoiding the over-approximation that FOLLOW sets introduce in SLR(1).

---

## 3. Challenges

### 3.1 Compiler ICE on g++ 5.3.0

The only C++ compiler available on the development machine was MinGW g++ 5.3.0 (from Anaconda). This version has a known internal compiler error (segmentation fault) when `<algorithm>` is included, triggered through the `<random>` header chain. Similarly, floating-point literals in certain contexts caused the same ICE.

**Solution:** All `<algorithm>` includes were removed and their uses replaced with manual equivalents:
- `std::sort` was unnecessary because `std::set<std::string>` is already ordered.
- `std::find` was replaced with a range loop.
- `std::max` was replaced with a ternary.
- Timing used integer arithmetic (`clock_t` / `CLOCKS_PER_SEC`) instead of floating-point to avoid the ICE entirely.
- C++17 structured bindings (`auto& [k, v]`) were replaced with `auto& p` + `p.first` / `p.second` for C++14 compatibility.

### 3.2 Correct LR(1) Lookahead Propagation

Computing `FIRST(β·a)` without constructing the concatenated string required careful generalisation of the standard `firstOfSequence` function. The edge case where every symbol in `β` can derive epsilon — making the entire tail nullable and therefore propagating the lookahead `a` — was easy to miss.

**Solution:** A dedicated `firstWithLookahead(seq, start, la)` helper was written that is identical to `firstOfSequence` except that instead of inserting `EPSILON` at the end, it inserts the lookahead terminal.

### 3.3 Child Order in Parse Trees During Reduction

When reducing `A → X1 X2 … Xn`, the symbols `Xn … X1` are on the top of the stack in reverse order (last-in first-out). Simply calling `node->children.push_back(treeStack.top()); treeStack.pop()` would produce a mirror-image tree.

**Solution:** `node->children` is pre-allocated to size `n`, and items are popped and assigned to indices `n-1, n-2, …, 0` in a single backwards loop:

```cpp
node->children.resize(n);
for (int k = n - 1; k >= 0; --k) {
    node->children[k] = treeStack.top();
    treeStack.pop(); stateStack.pop(); symStack.pop();
}
```

### 3.4 Epsilon Production Trees

An epsilon production `A → ε` has an empty RHS, so the pop loop does nothing. Without special handling, the resulting tree node would have no children, which is misleading.

**Solution:** After the loop, if `n == 0`, an explicit `epsilon` leaf node is added as the sole child, making the derivation visible in the parse tree.

---

## 4. Test Cases

### 4.1 Grammar 1 — Simple Expression (no multiplication)

```
Expr -> Expr + Term | Term
Term -> Factor
Factor -> id
```

Augmented start: `ExprPrime -> Expr`  
SLR(1) states: 7 | LR(1) states: 7 | Conflicts: 0 (both)

#### Valid Inputs

| Input | Result |
|-------|--------|
| `id` | ACCEPTED |
| `id + id` | ACCEPTED |
| `id + id + id` | ACCEPTED |
| `id + id + id + id` | ACCEPTED |
| `id + id + id + id + id` | ACCEPTED |

#### Invalid Inputs

| Input | Result | Reason |
|-------|--------|--------|
| `+` | REJECTED | `+` cannot begin an Expr |
| `id id` | REJECTED | Two consecutive ids with no operator |
| `id +` | REJECTED | Expr expected after `+` |

#### Sample Trace — `id + id`

```
Step Stack          Input           Action
1    0              id + id $       Shift 5
2    0 id 5         + id $          Reduce Factor -> id
3    0 Factor 3     + id $          Reduce Term -> Factor
4    0 Term 4       + id $          Reduce Expr -> Term
5    0 Expr 2       + id $          Shift 7
6    0 Expr 2 + 7   id $            Shift 5
7    0 Expr 2+7 id5 $               Reduce Factor -> id
8    ...            $               Reduce Term -> Factor
9    ...            $               Reduce Expr -> Expr + Term
10   0 Expr 2       $               Accept
```

---

### 4.2 Grammar 2 — Full Arithmetic Expressions

```
Expr   -> Expr + Term | Term
Term   -> Term * Factor | Factor
Factor -> ( Expr ) | id
```

SLR(1) states: 12 | LR(1) states: 22 | Conflicts: 0 (both)

#### Valid Inputs

| Input | Result |
|-------|--------|
| `id` | ACCEPTED |
| `id + id` | ACCEPTED |
| `id * id` | ACCEPTED |
| `id + id * id` | ACCEPTED |
| `( id + id ) * id` | ACCEPTED |
| `id * id + id` | ACCEPTED |
| `( id )` | ACCEPTED |
| `( id + id ) * ( id + id )` | ACCEPTED |
| `id + id + id` | ACCEPTED |
| `id + ( id * id )` | ACCEPTED |

#### Invalid Inputs

| Input | Result | Reason |
|-------|--------|--------|
| `+` | REJECTED | Operator at start |
| `id +` | REJECTED | Dangling operator |
| `id + + id` | REJECTED | Consecutive operators |
| `( id` | REJECTED | Unclosed parenthesis |
| `id )` | REJECTED | Unmatched close paren |
| `id id` | REJECTED | No operator between ids |
| `* id` | REJECTED | `*` cannot start expression |
| `( )` | REJECTED | Empty parentheses |
| `id + * id` | REJECTED | `*` cannot follow `+` |

#### Sample Trace — `id + id * id`

```
Step  Stack                              Input           Action
1     0                                  id+id*id $      Shift 5
2     0 id 5                             +id*id $        Reduce Factor -> id
3     0 Factor 3                         +id*id $        Reduce Term -> Factor
4     0 Term 4                           +id*id $        Reduce Expr -> Term
5     0 Expr 2                           +id*id $        Shift 7
6     0 Expr 2 + 7                       id*id $         Shift 5
7     0 Expr 2+7 id 5                    *id $           Reduce Factor -> id
8     0 Expr 2+7 Factor 3                *id $           Reduce Term -> Factor
9     0 Expr 2+7 Term 10                 *id $           Shift 8
10    0 Expr 2+7 Term10 * 8              id $            Shift 5
11    0 Expr 2+7 Term10*8 id 5           $               Reduce Factor -> id
12    0 Expr 2+7 Term10*8 Factor 11      $               Reduce Term -> Term * Factor
13    0 Expr 2+7 Term 10                 $               Reduce Expr -> Expr + Term
14    0 Expr 2                           $               Accept
```

Note: `*` binds tighter than `+` because the grammar requires `Term` to be fully reduced before `Expr + Term` reduces — the parser correctly delays the `id + …` reduction until after `id * id` is complete.

#### Parse Tree — `id + id * id`

```
Expr
|-- Expr
|   +-- Term
|       +-- Factor
|           +-- id
|-- +
+-- Term
    |-- Term
    |   +-- Factor
    |       +-- id
    |-- *
    +-- Factor
        +-- id
```

---

### 4.3 Grammar 3 — Dangling Else (ambiguous)

```
Stmt -> if Expr then Stmt else Stmt | if Expr then Stmt | other
Expr -> id
```

SLR(1) states: 10 | LR(1) states: 17 | Conflicts: 1 in **both** parsers  
This grammar is inherently ambiguous, so LR(1) cannot resolve the conflict either.

#### Valid Inputs (5 strings)

| Input | Result |
|-------|--------|
| `other` | ACCEPTED |
| `if id then other` | ACCEPTED |
| `if id then other else other` | ACCEPTED |
| `if id then if id then other else other` | ACCEPTED |
| `if id then if id then other else other else other` | ACCEPTED |

#### Conflict Detail

```
State 7, symbol 'else': shift/reduce — s8 vs r2
```

When the parser sees `if E then S` and the next token is `else`, it can either:
- **Shift** `else` → associates `else` with the *inner* `if` (standard C behaviour).
- **Reduce** `Stmt → if Expr then Stmt` → associates `else` with the *outer* `if`.

Both SLR(1) and LR(1) have this conflict because the grammar itself is ambiguous. The conflict is resolved by preferring shift, which implements the conventional "else belongs to the nearest if" rule.

---

### 4.4 Grammar 4 — Classic SLR(1) Conflict (LR(1) but NOT SLR(1))

```
Start -> Left = Right | Right
Left  -> * Right | id
Right -> Left
```

SLR(1) states: 10 | LR(1) states: 14 | SLR conflicts: **1** | LR(1) conflicts: **0**

This grammar is the canonical example demonstrating the difference between the two parsers.

#### Valid Inputs

| Input | Result (SLR) | Result (LR1) |
|-------|-------------|-------------|
| `id` | ACCEPTED | ACCEPTED |
| `* id` | ACCEPTED | ACCEPTED |
| `id = id` | ACCEPTED | ACCEPTED |
| `* id = id` | ACCEPTED | ACCEPTED |
| `* * id` | ACCEPTED | ACCEPTED |
| `* * id = * id` | ACCEPTED | ACCEPTED |

#### Why SLR(1) Fails

In the state reached after reducing `Left → id` (or `Left → * Right`), the parser has the item:

```
Right -> Left •
```

SLR(1) will reduce on any terminal in `FOLLOW(Right)`.  
`FOLLOW(Right) = { $, = }` because `=` can follow `Right` in `Start → Left = Right`.

But the same state also has a shift action on `=` from `Start → Left • = Right`.  
This creates a shift/reduce conflict on `=`.

#### Why LR(1) Succeeds

LR(1) splits the state by context. The item in the state relevant to this reduction is:

```
[Right -> Left •, $]
```

The lookahead is `$` only — the parser knows that in this context, `Right` is not being followed by `=`. Therefore no conflict arises: shift on `=`, reduce on `$`.

#### Comparison Table

```
                           SLR(1)    LR(1)
States generated              10        14
ACTION entries                17        22
GOTO entries                   7         9
Conflicts                      1         0
Grammar is valid?       NO (conflict)   YES
```

---

### 4.5 Grammar 5 — Epsilon Productions (edge case)

```
Prog     -> Stmt Prog | epsilon
Stmt     -> id = Expr ;
Expr     -> Expr + id | id
```

SLR(1) states: 11 | LR(1) states: 11 | Conflicts: 0 (both)

`Prog → epsilon` allows an empty program. FIRST(Prog) = { id, epsilon }.

#### Valid Inputs

| Input | Result |
|-------|--------|
| `id = id ;` | ACCEPTED |
| `id = id + id ;` | ACCEPTED |
| `id = id ; id = id ;` | ACCEPTED |

#### Parse Tree — `id = id ;`

```
Prog
|-- Stmt
|   |-- id
|   |-- =
|   |-- Expr
|   |   +-- id
|   +-- ;
+-- Prog
    +-- epsilon
```

The second `Prog` derives epsilon, and this is explicitly shown as an `epsilon` leaf in the tree.

---

## 5. Comparison Analysis

### 5.1 SLR(1) vs LR(1) Parsing Power

SLR(1) is strictly less powerful than LR(1). Every SLR(1) grammar is LR(1), but not vice versa. The difference is in how reduce actions are determined:

- **SLR(1)** reduces on `FOLLOW(A)` — the set of all terminals that can ever follow `A` anywhere in the grammar. This is a global over-approximation.
- **LR(1)** reduces only on the specific lookahead carried by the item — a per-context, precise decision.

The canonical example (Grammar 4) demonstrates this: `FOLLOW(Right) = {$, =}` causes SLR(1) to try to reduce in a context where shifting is the only correct action. LR(1) avoids this by tracking the exact context in which `Right` was begun.

### 5.2 Number of States Comparison

| Grammar | SLR(1) | LR(1) | Increase |
|---------|--------|-------|----------|
| Grammar 1 (simple) | 7 | 7 | 0% |
| Grammar 2 (arithmetic) | 12 | 22 | +83% |
| Grammar 3 (dangling else) | 10 | 17 | +70% |
| Grammar 4 (conflict) | 10 | 14 | +40% |
| Grammar 5 (epsilon) | 11 | 11 | 0% |

For simple grammars (Grammars 1 and 5), the state counts are identical because every state has only one possible lookahead context, so splitting adds nothing. For grammars with shared non-terminals used in multiple contexts (like Grammar 2, where `Factor` is used inside both `Term` and parenthesised expressions at different nesting levels), LR(1) splits states that SLR(1) merges.

### 5.3 Table Size (Memory Usage)

| Grammar | SLR(1) ACTION | LR(1) ACTION | SLR(1) GOTO | LR(1) GOTO |
|---------|--------------|-------------|------------|-----------|
| Grammar 1 | 12 | 12 | 5 | 5 |
| Grammar 2 | 36 | 56 | 9 | 15 |
| Grammar 3 | 16 | 26 | 4 | 7 |
| Grammar 4 | 17 | 22 | 7 | 9 |

Memory usage scales linearly with the number of non-empty table entries. LR(1) uses roughly 1.5× the ACTION entries and 1.7× the GOTO entries compared to SLR(1) for Grammar 2. In practice, sparse matrix representations (as used here with `std::map`) make the absolute memory difference small. For production compilers handling thousands of states, LALR(1) — which merges LR(1) states that have identical cores — is preferred as a middle ground.

### 5.4 Construction Time

All grammars were constructed in under 2 ms on the test machine. The time is dominated by the closure computation (which is O(states × productions × items) per iteration). For practical grammars with thousands of productions (e.g., C++ or Java), LR(1) construction would be significantly slower than SLR(1) due to the state explosion. LALR(1) mitigates this while retaining most of LR(1)'s conflict-resolution power.

| Grammar | SLR(1) build | LR(1) build |
|---------|-------------|------------|
| Grammar 2 | ~1 ms | ~1 ms |
| Grammar 4 (conflict) | ~1 ms | ~1 ms |

---

## 6. Sample Outputs

### 6.1 Program Execution — Augmented Grammar and FIRST/FOLLOW (Grammar 2)

The screenshot below shows the augmented grammar produced for Grammar 2, along with the computed FIRST and FOLLOW sets used by the SLR(1) table builder.

![Augmented grammar, FIRST and FOLLOW sets for Grammar 2](screenshots/ss01-augmented-grammar.png)

---

### 6.2 LR(0) Item Sets (Grammar 2)

The canonical collection contains 12 states. The screenshot shows states I0 through I5, illustrating how CLOSURE expands each kernel item.

![LR(0) canonical collection states I0–I5 for Grammar 2 (part 1)](screenshots/ss02-lr0-items-grammar2.png)

![LR(0) canonical collection states I6–I11 for Grammar 2 (part 2)](screenshots/ss02-lr0-items-grammar2-2.png)

---

### 6.3 LR(1) Item Sets — State I0 (Grammar 4)

The screenshot shows I0 for the conflict grammar. Each item carries an explicit lookahead. Notice `Left` appears with two separate lookaheads (`$` and `=`), reflecting the two contexts in which it can be reduced — this is the information that allows LR(1) to resolve the conflict.

![LR(1) item set I0 for Grammar 4 (conflict grammar), showing per-item lookaheads](screenshots/ss03-lr1-items-grammar4.png)

---

### 6.4 SLR(1) Parsing Table (Grammar 2)

The complete ACTION/GOTO table for Grammar 2. All 12 states are shown. There are no conflicts — every cell has at most one entry.

![Complete SLR(1) ACTION/GOTO table for Grammar 2](screenshots/ss04-slr-table-grammar2.png)

---

### 6.5 Parse Trace — `id + id * id` (Grammar 2, SLR(1))

The step-by-step trace demonstrating correct operator precedence. The `*` operator is reduced before `+` because the grammar enforces it structurally through `Term` being reduced before `Expr`.

![SLR(1) parse trace for input "id + id * id" on Grammar 2](screenshots/ss05-parse-trace-grammar2.png)

---

### 6.6 Conflict Example — SLR(1) Table (Grammar 4)

The SLR(1) table for the conflict grammar. State 2 on symbol `=` shows both `s8` (shift) and `r5` (reduce `Right → Left`), which is the shift/reduce conflict. The program reports it explicitly and marks the grammar as **not SLR(1)**.

![SLR(1) parsing table for Grammar 4 showing the shift/reduce conflict in State 2](screenshots/ss06-slr-table-conflict.png)

---

### 6.7 Conflict Resolution — LR(1) Table (Grammar 4)

The LR(1) table for the same grammar. State 2 now has `s8` only on `=` and `r5` only on `$` — no overlap. The program reports **no conflicts** and marks the grammar as a valid LR(1) grammar.

![LR(1) parsing table for Grammar 4 showing the conflict resolved](screenshots/ss07-lr1-table-conflict.png)

---

### 6.8 Parser Comparison Output (Grammar 4)

The side-by-side comparison produced by `--compare`, showing the state count increase, table entry counts, and the conflict summary for both parsers.

![Comparison output for Grammar 4: SLR(1) has 1 conflict, LR(1) has 0](screenshots/ss08-comparison-conflict.png)

---

### 6.9 Parse Trees (Grammar 2)

Parse trees for accepted strings on Grammar 2. The tree for `id + id * id` correctly shows `id * id` grouped under a `Term` subtree, reflecting higher precedence of `*` over `+`.

![Parse trees generated for Grammar 2 inputs (part 1)](screenshots/ss09-parse-trees-grammar2.png)

![Parse trees generated for Grammar 2 inputs (part 2)](screenshots/ss09-parse-trees-grammar2-2.png)

---

### 6.10 Parse Tree with Epsilon Leaf (Grammar 5)

The tree for `id = id ;` on Grammar 5 (epsilon grammar). The trailing `Prog → epsilon` reduction is shown explicitly as an `epsilon` leaf node, making the derivation visible.

![Parse tree for Grammar 5 showing epsilon leaf node](screenshots/ss10-parse-tree-epsilon.png)

---

## 7. Conclusion

### Lessons Learned

**1. Lookahead precision is the key difference.**  
SLR(1) uses a global over-approximation (FOLLOW sets) that causes conflicts on grammars that are otherwise unambiguous. LR(1) carries context-sensitive lookaheads through closure, which is the minimum information needed to make correct reduce decisions. Grammar 4 makes this concrete: one extra bit of context (`$` vs `=`) is all that is needed to resolve the conflict.

**2. The CLOSURE operation is the core of both parsers.**  
Nearly all correctness bugs during implementation were in the CLOSURE function — specifically in the LR(1) case around lookahead propagation when the tail `β` of a production can derive epsilon. Getting `firstWithLookahead` correct was the most subtle part of the project.

**3. More states does not mean worse parser.**  
LR(1) generates more states than SLR(1) for the same grammar, but this extra work buys increased parsing power and fewer spurious conflicts. For real language grammars (C, Java), LALR(1) — which merges LR(1) states with identical cores — captures most of this power while staying close to SLR(1) in size.

**4. Parse trees fall out naturally from the reduction sequence.**  
Maintaining a parallel tree-node stack alongside the state and symbol stacks requires only that reductions create a new node and adopt the top-n nodes as children. This required careful attention to the reversal of children (stack is LIFO, children should be left-to-right).

**5. Epsilon productions must be explicitly handled.**  
An epsilon reduction pops zero items from the stack but still creates a tree node. Without adding an explicit `epsilon` leaf, the resulting tree node would appear to have no children, hiding the derivation step.

### Insights

- **SLR(1) is practical for most programming language grammars** that are carefully engineered to avoid the FOLLOW-set ambiguities. Yacc/Bison use LALR(1) which is similarly sized to SLR(1) but as powerful as LR(1).
- **Ambiguous grammars (like the dangling else) cannot be parsed conflict-free by any LR variant.** The conflict must be resolved by a tie-breaking rule (prefer shift) or by grammar transformation.
- **The canonical SLR(1) conflict grammar** (`Start → L=R | R; L → *R | id; R → L`) is worth memorising — it is the standard counterexample used to explain why SLR(1) is strictly weaker than LR(1).
