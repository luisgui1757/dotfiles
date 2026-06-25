def summarize_counts(items:list[str])->dict[str,int]:
    counts:dict[str,int]={}
    for item in items:
        counts[item]=counts.get(item,0)+1
    return counts
