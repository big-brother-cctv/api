package com.bigBrother.api.repositories;

import com.bigBrother.api.models.CameraModel;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.Optional;

@Repository
public interface CameraRepository extends JpaRepository<CameraModel, Long> {
    Optional<CameraModel> findByName(String name);
}