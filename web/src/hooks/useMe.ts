import { useQuery } from '@tanstack/react-query'
import { me } from '@/api/user'

// Retry policy is set at the QueryClient level (lib/queryClient.ts) so tests
// can override it globally. This hook only configures what's unique to /me.
export function useMe() {
  return useQuery({
    queryKey: ['me'],
    queryFn: me,
    staleTime: 5 * 60_000,
  })
}
