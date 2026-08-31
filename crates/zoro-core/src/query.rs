use crate::model::Entry;

/// 一个命中候选：条目 + 分数 + 命中位置（字节区间，相对 index_text）。
#[derive(Debug, Clone)]
pub struct Candidate {
    pub entry: Entry,
    pub score: i64,
    pub matches: Vec<(usize, usize)>,
}

/// P0 内置的极简子序列模糊匹配器（大小写不敏感）。
///
/// 后续可用 `nucleo` 替换为真正的模糊匹配 + 排序；此处只保证核心查询语义可用，
/// 且不引入外部依赖。返回 `None` 表示不匹配。
pub fn fuzzy_match(query: &str, text: &str) -> Option<(i64, Vec<(usize, usize)>)> {
    if query.is_empty() {
        return Some((0, Vec::new()));
    }
    let q: Vec<char> = query.chars().collect();
    let t: Vec<char> = text.chars().collect();
    let t_lower: Vec<char> = text.to_lowercase().chars().collect();
    let q_lower: Vec<char> = query.to_lowercase().chars().collect();

    let mut qi = 0usize;
    let mut prev = None;
    let mut score: i64 = 0;
    let mut ranges: Vec<(usize, usize)> = Vec::new();
    let mut range_start: Option<usize> = None;
    let mut last_ti = 0usize;

    for (ti, _tc) in t.iter().enumerate() {
        if qi == q.len() {
            break;
        }
        let qc = q_lower[qi];
        let tc_lower = t_lower[ti];
        if tc_lower == qc {
            if qi == 0 {
                // 首个命中位置越靠前分越高
                score -= ti as i64;
            }
            match prev {
                Some(p) if p + 1 == ti => {
                    score += 8; // 连续
                }
                _ => {
                    score -= (ti.saturating_sub(last_ti)) as i64; // 跳跃惩罚
                }
            }
            prev = Some(ti);
            last_ti = ti;
            if range_start.is_none() {
                range_start = Some(ti);
            }
            qi += 1;
            if qi == q.len() {
                ranges.push((range_start.unwrap(), ti + 1));
                break;
            }
        } else if range_start.is_some() {
            ranges.push((range_start.unwrap(), ti));
            range_start = None;
        }
    }

    if qi < q.len() {
        return None;
    }
    // 短的 index 文本优先（更聚焦）。
    score -= (t.len() as i64) / 4;
    Some((score, ranges))
}

/// 在条目集上执行查询，按分数降序返回。
pub fn search(entries: &[Entry], query: &str) -> Vec<Candidate> {
    let mut out = Vec::new();
    for e in entries {
        let text = e.index_text();
        if let Some((score, matches)) = fuzzy_match(query, &text) {
            out.push(Candidate {
                entry: e.clone(),
                score,
                matches,
            });
        }
    }
    out.sort_by(|a, b| b.score.cmp(&a.score));
    out
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn subsequence_matches() {
        let (_score, ranges) = fuzzy_match("gp", "git pull").unwrap();
        assert!(!ranges.is_empty());
    }

    #[test]
    fn no_match() {
        assert!(fuzzy_match("zz", "git pull").is_none());
    }

    #[test]
    fn case_insensitive() {
        assert!(fuzzy_match("GIT", "git pull").is_some());
    }
}
