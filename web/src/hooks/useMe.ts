import { useQuery } from '@tanstack/react-query'
import { me } from '@/api/user'

export function useMe() {
  return useQuery({
    queryKey: ['me'],
    queryFn: me,
    staleTime: 5 * 60_000,
    retry: 1,
  })
}
