package service

import "context"

func (s *UpdateService) SyncSecuritiesList(ctx context.Context) ([]string, error) {
	if err := s.SyncSecurities(ctx); err != nil {
		return nil, err
	}
	list, err := s.secRepo.List(ctx, s.market)
	if err != nil {
		return nil, err
	}
	codes := make([]string, len(list))
	for i, sec := range list {
		codes[i] = sec.Code
	}
	return codes, nil
}
