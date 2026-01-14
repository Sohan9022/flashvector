package storage

import (
	"os"
	"encoding/gob"
)

func (s *Store) SaveSnapShot(path string) error{
	s.mu.RLock()
	defer s.mu.RUnlock()

	file,err := os.Create(path)

	if err != nil{
		return err
	}

	defer file.Close()

	encoder := gob.NewEncoder(file)

	return encoder.Encode(s.data)

}

func (s *Store) LoadSnapshot(path string) error{
	file,err := os.Open(path)

	if err != nil{
		if os.IsNotExist(err){
			return nil
		}
		return err
	}

	defer file.Close()

	decoder := gob.NewDecoder(file)

	data := make(map[string][]byte)

	if err := decoder.Decode(&data);err != nil{
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = data
	s.index.RebuildFromData(data)
	return nil
	
}