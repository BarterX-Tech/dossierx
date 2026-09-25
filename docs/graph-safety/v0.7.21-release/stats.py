import statistics as st, sys
def ms(s):
    return float(s[:-2]) if s.endswith('ms') else float(s[:-1]) * 1000
path = sys.argv[1]
d = {}; a = {}; ld = []
for l in open(path):
    v, i, res, el, al, cb, vb, load = l.rstrip('\n').split('\t')
    d.setdefault(v, []).append(ms(el)); a.setdefault(v, []).append(int(al)); ld.append(float(load))
for v in d:
    x = sorted(d[v])
    print(v, 'n', len(x), 'min %.0f median %.0f max %.0f ms' % (x[0], st.median(x), x[-1]), 'alloc min %d max %d' % (min(a[v]), max(a[v])))
print('load1 range', min(ld), max(ld))
