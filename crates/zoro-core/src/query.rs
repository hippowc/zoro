use std::path::PathBuf;

use crate::model::Block;

/// 一个命中候选（跨库聚合后的统一模型）。
///
/// 三分离语义：命中/匹配字段（`index`）≠ 显示字段（`title`）≠ 载荷
/// （`(library, path, start)` 现场截取）。
#[derive(Debug, Clone)]
pub struct Candidate {
    pub library: String,
    pub title: String,
    pub index: String,
    pub path: PathBuf,
    pub start: usize,
    pub score: i64,
    /// 命中位置（字符区间，相对 `index`）。
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

/// 在指定库的条目集上执行查询，按分数降序返回。
pub fn search(blocks: &[Block], library: &str, query: &str) -> Vec<Candidate> {
    let mut out = Vec::new();
    for e in blocks {
        let text = e.index_text();
        if let Some((score, matches)) = fuzzy_match(query, &text) {
            out.push(Candidate {
                library: library.to_string(),
                title: e.title.clone(),
                index: text.clone(),
                path: e.path.clone(),
                start: e.start,
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

    #[test]
    fn search_tags_candidate_with_library() {
        let e = Block {
            title: "Git pull".into(),
            terms: vec!["git".into(), "pull".into()],
            ..Default::default()
        };
        let hits = search(&[e], "my-lib", "gp");
        assert_eq!(hits.len(), 1);
        assert_eq!(hits[0].library, "my-lib");
        assert_eq!(hits[0].title, "Git pull");
    }
}
