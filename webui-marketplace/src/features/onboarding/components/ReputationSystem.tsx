import { useEffect, useState } from 'react';
import { RefreshCw } from 'lucide-react';
import { useSessionStore } from '../../../app/store/sessionStore';
import { Button } from '../../../components/ui/Button';
import { EmptyState } from '../../../components/ui/EmptyState';
import { cn } from '../../../lib/cn';
import { getMe, getReputation } from '../../../lib/api/client';
import type { Rating, ReputationSummary, User } from '../../../types/domain';
import { RatingForm } from './RatingForm';
import { RatingHistory } from './RatingHistory';
import { ReputationScoreCard } from './ReputationScoreCard';

export interface ReputationSystemProps {
  className?: string;
}

export function ReputationSystem({ className }: ReputationSystemProps) {
  const currentUser = useSessionStore((state) => state.currentUser);
  const [user, setUser] = useState<User | null>(currentUser);
  const [summary, setSummary] = useState<ReputationSummary | null>(null);
  const [ratings, setRatings] = useState<Rating[]>([]);
  const [usedFallback, setUsedFallback] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let cancelled = false;

    async function load() {
      setLoading(true);
      setError(null);

      try {
        const profile = currentUser ?? (await getMe());
        if (cancelled) {
          return;
        }
        setUser(profile);

        try {
          const reputation = await getReputation(profile.id);
          if (cancelled) {
            return;
          }
          setSummary(reputation);
          setRatings(reputation.ratings);
          setUsedFallback(reputation.dimensions.length === 0);
        } catch {
          // 实时声望不可用时，用 getMe() 的 reputation 概览兜底。
          if (cancelled) {
            return;
          }
          setSummary(null);
          setRatings([]);
          setUsedFallback(true);
        }
      } catch (loadError) {
        if (cancelled) {
          return;
        }
        setError(
          loadError instanceof Error ? loadError.message : '声望数据加载失败。',
        );
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    void load();
    return () => {
      cancelled = true;
    };
  }, [currentUser?.id, reloadKey]);

  function handleRatingSubmitted(rating: Rating) {
    setRatings((prev) => [
      rating,
      ...prev.filter((item) => item.id !== rating.id),
    ]);
  }

  if (error) {
    return (
      <EmptyState
        className={className}
        title="声望数据加载失败"
        description={error}
        action={
          <Button
            variant="outline"
            size="sm"
            onClick={() => setReloadKey((key) => key + 1)}
          >
            <RefreshCw className="h-3.5 w-3.5" />
            重新加载
          </Button>
        }
      />
    );
  }

  return (
    <div className={cn('space-y-4', className)}>
      <ReputationScoreCard
        summary={summary}
        fallbackUser={user}
        usedFallback={usedFallback}
        ratingCount={ratings.length}
        loading={loading}
      />

      <div className="grid gap-4 lg:grid-cols-2">
        <RatingForm
          direction="EMPLOYER_TO_COMMANDER"
          onSubmitted={handleRatingSubmitted}
        />
        <RatingForm
          direction="COMMANDER_TO_EMPLOYER"
          onSubmitted={handleRatingSubmitted}
        />
      </div>

      <RatingHistory ratings={ratings} loading={loading} />
    </div>
  );
}
